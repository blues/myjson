#! /bin/bash

# Note that when we run the server we use sudo because it is a Linux
# design constraint that non-supervisors cannot listen on ports
# less than 1024. This was discovered when running on GCS, which
# by default runs our code unprivileged.

# Mount the EFS volume, assuming that we're now running under AWS
while [ ! -d "$HOME/efs/notehub" ]; do
    pushd "$HOME"
    sudo mount -t nfs4 -o nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2 fs-b0df64f9.efs.us-east-1.amazonaws.com:/ efs
    popd
    sleep 5s
done

set -v
git reset --hard
git pull git@github.com:rayozzie/notehub
go get -u
go build

sudo ./notehub $HOME/efs/notehub
