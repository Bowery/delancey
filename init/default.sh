#!/bin/bash

versionurl="http://bowery.sh.s3.amazonaws.com/VERSION"
mkdir -p ~/.bowery
cd ~/.bowery

if [ -f bowery-agent.pid ] ; then
  PID=`cat bowery-agent.pid 2>/dev/null`
  if [ -z "$PID" ] || ! ps "$PID" >/dev/null ; then
    echo "bowery-agent with pid=${PID} is not running - removing old pidfile"
    unset PID
    rm -f bowery-agent.pid
  else
    echo "Killing current bowery-agent with pid=${PID}..."
    sudo kill -- -$PID 2>/dev/null
  fi
fi
if [ -f bowery-agent.new ] ; then
  mv bowery-agent.new bowery-agent
fi

sleep 2
/usr/local/bin/bowery-updater "${versionurl}" "" /usr/local/bin/bowery-agent &> bowery-agent.log &
echo $! > bowery-agent.pid
