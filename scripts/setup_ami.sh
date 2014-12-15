#!/bin/bash
apt-get update
apt-get install -y docker.io git-core vim

wget http://bowery.sh/bowery-agent
chmod +x bowery-agent
./bowery-agent &> /home/ubuntu/bowery-agent-debug.log
