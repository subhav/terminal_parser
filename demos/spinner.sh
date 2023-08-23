#!/usr/bin/env sh

# Mock progress spinner, taken from
# https://stackoverflow.com/questions/238073/how-to-add-a-progress-bar-to-a-shell-script
while : ; do
  for s in / - \\ \| ; do
    printf "\r$s"
    sleep .1
  done
done
