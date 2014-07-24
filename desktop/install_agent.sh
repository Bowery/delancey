#!/bin/bash
# Bowery Delancy agent install script
# bash -c "$(curl -s bowery.sh)"

set -e

echo "Thanks for using Bowery!"

dir=/tmp/bowery
mkdir -p $dir

# figure out operating system...
if [[ $OSTYPE == *linux* ]]
then
    OS=linux
else
    echo "Sorry, we do not support this server's operating system."
    exit 1
fi

# figure out architecture...
if [[ $(uname -m) == "x86_64" ]]
then
    ARCH=amd64
elif [[ $(uname -m) == *arm* ]]
then
    ARCH=arm
else
    ARCH=368
fi

bucket=bowery.sh
s3url=s3.amazonaws.com
VERSION=$(curl -s http://${bucket}.${s3url}/VERSION)

printf "Downloading agent... "
curl -so $dir/bowery-agent.tar.gz http://${bucket}.${s3url}/${VERSION}_${OS}_${ARCH}.tar.gz
printf "Installing... "
tar -xzf $dir/bowery-agent.tar.gz
mv delancey $dir/bowery-agent
sudo mv $dir/bowery-agent /usr/local/bin
echo "Done!"

printf "Setting up daemon... "
curl -so $dir/bowery-agent.conf http://${bucket}.${s3url}/delancey.conf
sudo mv $dir/bowery-agent.conf /etc/init/
sudo service bowery-agent start
echo "Done!"
