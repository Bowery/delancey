# Bowery/Base Image
FROM       ubuntu:12.04

RUN        echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
RUN        apt-get update
RUN        apt-get upgrade
RUN        apt-get install -y openssh-server supervisor make git-core

RUN        mkdir -p /application
RUN        mkdir -p /satellite
RUN        mkdir -p /var/run/sshd
RUN        mkdir -p /var/log/supervisor

RUN        echo 'root:password' | chpasswd
ADD        supervisord.conf /etc/supervisor/conf.d/supervisord.conf
ADD        satellite /satellite/satellite

EXPOSE     22 80 3001
CMD        ["/usr/bin/supervisord"]
