// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Image file extensions recognized by the photo viewer.
var photoExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
}

// Maximum number of retained photos per target directory.
const configMaxPhotos = 10

// How often the photo purge goroutine sweeps every photo directory.
const photoPurgeInterval = 30 * time.Second

// photoEntry is a single image file with the modification time we sort on.
type photoEntry struct {
	name string
	mod  time.Time
}

// listPhotosByMTime returns every image file in the target directory sorted
// newest-first by modification time. Returns nil on any error.
func listPhotosByMTime(target string) []photoEntry {
	dir := filepath.Join(configDataDirectory, target)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var imgs []photoEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !photoExtensions[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		imgs = append(imgs, photoEntry{e.Name(), info.ModTime()})
	}
	sort.Slice(imgs, func(i, j int) bool {
		return imgs[i].mod.After(imgs[j].mod)
	})
	return imgs
}

// isPhotoDirectory returns true if the target directory exists and contains
// at least one file with a recognized image extension.
func isPhotoDirectory(target string) bool {
	dir := filepath.Join(configDataDirectory, target)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if photoExtensions[strings.ToLower(filepath.Ext(e.Name()))] {
			return true
		}
	}
	return false
}

// latestPhoto returns the name (and modification time) of the most recently
// modified image file in the target directory, or "" if none exists.
func latestPhoto(target string) (name string, mod time.Time) {
	imgs := listPhotosByMTime(target)
	if len(imgs) == 0 {
		return
	}
	return imgs[0].name, imgs[0].mod
}

// purgePhotos deletes all but the `keep` most recently modified image files
// from the target directory. Silent no-op if the directory has fewer than
// `keep` images.
func purgePhotos(target string, keep int) {
	if keep < 0 {
		keep = 0
	}
	imgs := listPhotosByMTime(target)
	if len(imgs) <= keep {
		return
	}
	dir := filepath.Join(configDataDirectory, target)
	for _, img := range imgs[keep:] {
		path := filepath.Join(dir, img.name)
		if err := os.Remove(path); err != nil {
			fmt.Printf("purge photo %s/%s: %s\n", target, img.name, err)
		} else {
			fmt.Printf("purged photo %s/%s\n", target, img.name)
		}
	}
}

// purgePhotosLoop runs forever, periodically trimming every photo directory
// under configDataDirectory down to the latest configMaxPhotos images.
// It uses tailTargets() to discover subdirectories and isPhotoDirectory() to
// skip JSON-only targets, so a single sweep handles "photos", "camnote", etc.
func purgePhotosLoop() {
	for {
		time.Sleep(photoPurgeInterval)
		for _, target := range tailTargets() {
			if !isPhotoDirectory(target) {
				continue
			}
			purgePhotos(target, configMaxPhotos)
		}
	}
}

// photoLatest returns the newest image filename as plain text, suitable
// for the viewer page's polling loop.
func photoLatest(httpRsp http.ResponseWriter, target string) {
	name, _ := latestPhoto(target)
	httpRsp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	httpRsp.Header().Set("Cache-Control", "no-store")
	httpRsp.Write([]byte(name))
}

// photoViewer serves an HTML page that displays the latest photo in the
// target directory and polls for updates so the browser refreshes the
// moment a new image arrives.
func photoViewer(httpRsp http.ResponseWriter, target string) {
	httpRsp.Header().Set("Content-Type", "text/html; charset=utf-8")
	httpRsp.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(httpRsp, photoViewerHTML, target)
}

// photoViewerHTML is the viewer page. The single %[1]s/%[1]q placeholder is
// the (already-cleaned) target directory name, which is safe to embed.
const photoViewerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%[1]s</title>
<style>
  html, body { margin: 0; padding: 0; height: 100%%; background: #000; }
  body { display: flex; align-items: center; justify-content: center;
         font-family: -apple-system, system-ui, sans-serif; }
  img { max-width: 100vw; max-height: 100vh; object-fit: contain;
        opacity: 0; transition: opacity .15s ease-in; }
  img.loaded { opacity: 1; }
  #status { position: fixed; top: 8px; left: 8px; color: #aaa;
            font: 12px/1.3 ui-monospace, Menlo, monospace;
            background: rgba(0,0,0,0.55); padding: 5px 7px;
            border-radius: 4px; pointer-events: none; }
  #empty { color: #888; font-size: 14px; }
</style>
</head>
<body>
<img id="photo" alt="">
<div id="empty">waiting for first photo…</div>
<div id="status">connecting…</div>
<script>
(function() {
  var target = %[1]q;
  var img = document.getElementById('photo');
  var empty = document.getElementById('empty');
  var status = document.getElementById('status');
  var current = '';
  var inflight = false;

  img.addEventListener('load', function() {
    img.classList.add('loaded');
    if (empty) { empty.style.display = 'none'; }
  });

  async function poll() {
    if (inflight) { return; }
    inflight = true;
    try {
      var r = await fetch('/' + target + '?latest=1', { cache: 'no-store' });
      if (r.ok) {
        var name = (await r.text()).trim();
        if (name && name !== current) {
          current = name;
          img.classList.remove('loaded');
          img.src = '/' + target + '/' + encodeURIComponent(name);
          status.textContent = name;
        } else if (!name) {
          status.textContent = 'no photos yet';
        }
      } else {
        status.textContent = 'http ' + r.status;
      }
    } catch (e) {
      status.textContent = 'error: ' + e;
    } finally {
      inflight = false;
    }
  }

  poll();
  setInterval(poll, 400);
})();
</script>
</body>
</html>
`
