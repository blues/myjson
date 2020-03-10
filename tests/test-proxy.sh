#!/bin/bash
for (( i=1; i<=$1; i++ ))
  do
    echo $i/$1
	curl -L 'https://notecard.live/proxy?url=http%3A%2F%2F04a09601bb0ccaa3381deda1ae3c73c6.balena-devices.com%2Freq%3Fkey%3D123ABC%3Bid%3D11' -d '{"req":"card.voltage"}'
  done




