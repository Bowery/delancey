#!/bin/bash
# Bowery Delancey agent install script
# curl -fsS bowery.sh | bash

set -e

# for colorful terminal output
function colecho {
    local GREEN="\033[32m"
    local RED="\033[91m"
    local CYAN="\033[36m"
    local NONE="\033[0m"

    case $1 in
        -g|--green)
            printf "$GREEN${*:2}" ;;
        -r|--red)
            printf "$RED${*:2}" ;;
        -c|--cyan)
            printf "$CYAN${*:2}" ;;
        *)
            printf "${*}" ;;
    esac

    printf "$NONE\n"
}

function on_error {
    colecho -r "Bowery agent installation failed."
}
trap on_error ERR

function script_error {
    if [[ -n $ERR_STR ]]; then echo $ERR_STR; fi
}
trap script_error EXIT

OS_ERR="Sorry, we do not support this server's operating system."
LNX_DSTR_DTCTN_ERR="Sorry, we cannot detect this server's Linux distribution."

function os_error {
    ERR_STR="$OS_ERR"

    if [[ -n $OS ]]; then ERR_STR="$ERR_STR $OS"; fi
    if [[ -n $NAME ]]; then ERR_STR="$ERR_STR $NAME"; fi

    on_error
    exit 1
}

# linux_install is called after the binaries are installed. Initiates the service
function linux_install {
    if [[ -e /etc/os-release ]] # pretty much on everything except VMs
    then
        # exposes distro as $NAME
        . /etc/os-release
    elif [[ -e /proc/version ]] # definitely everywhere
    then
        NAME="$(cat /proc/version)"
    else
        ERR_STR=$LNX_DSTR_ERR
        exit 1
    fi

    # for better error messaging
    case $NAME in
        *Fedora*)
            NAME=Fedora ;;
        *CentOS*)
            NAME=CentOS ;;
        *Red\ Hat*)
            NAME=RedHat ;;
        *SUSE*)
            NAME=SUSE ;;
        *Ubuntu*)
            NAME=Ubuntu ;;
        *Debian*)
            NAME=Debian ;;
        *)
            os_error ;;
    esac

    case $NAME in
        "Fedora"|"RedHat"|"CentOS") # logs `journalctl _SYSTEMD_UNIT=bowery-agent.service`
            curl -fsSo $dir/bowery-agent.service http://${bucket}.${s3url}/systemd.agent.service
            sudo mv $dir/bowery-agent.service /etc/systemd/user/
            sudo systemctl disable /etc/systemd/user/bowery-agent.service # just in case
            sudo systemctl enable /etc/systemd/user/bowery-agent.service
            sudo systemctl start bowery-agent.service > /dev/null ;;
        "Ubuntu")
            curl -fsSo $dir/bowery-agent.conf http://${bucket}.${s3url}/upstart.agent.conf
            sudo mv $dir/bowery-agent.conf /etc/init/
            sudo service bowery-agent start > /dev/null ;;
        "Debian") # logs /var/log/bowery-agent.log
            curl -fsSo $dir/bowery-agent http://${bucket}.${s3url}/sysvinit.agent
            sudo mv $dir/bowery-agent /etc/init.d/
            sudo chmod 755 /etc/init.d/bowery-agent
            sudo ln -sf /etc/init.d/bowery-agent /etc/rc3.d/S95bowery-agent
            sudo /etc/init.d/bowery-agent start > /dev/null ;;
        "SUSE")
            os_error ;;
    esac
}

function darwin_install {
    # curl -fsSo $dir/com.bowery.bowery.plist http://${bucket}.${s3url}/com.bowery.bowery.plist
    # sudo mv $dir/com.bowery.bowery.plist /Library/LaunchAgents
    # sudo launchctl load -Fw /Library/LaunchAgents/com.bowery.bowery.plist
    default_install
}

function solaris_install {
    # Commands: http://wiki.smartos.org/display/DOC/Basic+SMF+Commands
    # File Format: http://wiki.smartos.org/display/DOC/Building+Manifests
    # see logs at: /var/svc/log/bowery-bowery:default.log
    curl -fsSo $dir/bowery.xml http://${bucket}.${s3url}/bowery.xml
    svccfg validate $dir/bowery.xml
    svccfg import $dir/bowery.xml

    svcadm disable /bowery/bowery
    svcadm enable /bowery/bowery
}

function default_install {
    curl -fsSo $dir/bowery-run http://${bucket}.${s3url}/default.sh
    sudo mv $dir/bowery-run /usr/local/bin/
    sudo chmod 755 /usr/local/bin/bowery-run
    sudo /usr/local/bin/bowery-run
}

colecho -g "Thanks for using Bowery!"

dir=/tmp/bowery
mkdir -p $dir

# figure out operating system...
case $OSTYPE in
    *linux*)
        OS=linux ;;
    *darwin*)
        OS=darwin ;;
    *solaris*)
        OS=solaris ;;
    *)
        os_error ;;
esac

# figure out architecture...
case "$(uname -m)" in
    "x86_64")
        ARCH=amd64 ;;
    *arm*)
        ARCH=arm ;;
    *)
        ARCH=386 ;;
esac


# Assume Solaris is amd64. uname -m wont work for SmartOS
if [ $OS == 'solaris' ]; then ARCH=amd64; fi;

bucket=bowery.sh
s3url=s3.amazonaws.com
VERSION=$(curl -fsS http://${bucket}.${s3url}/VERSION | head -n 1)

printf "Downloading agent... "
curl -fsSo $dir/bowery-agent.tar.gz http://${bucket}.${s3url}/${VERSION}_${OS}_${ARCH}.tar.gz
printf "Installing... "
tar -xzf $dir/bowery-agent.tar.gz
sudo mv * /usr/local/bin
colecho -c "Done!"

printf "Setting up daemon... "
case $OS in
    "linux")
        linux_install ;;
    "darwin")
        darwin_install ;;
    "solaris")
        solaris_install ;;
    *)
        default_install ;;
esac

colecho -c "Done!"
exit 0
