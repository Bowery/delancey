Vagrant.configure("2") do |config|
  config.vm.box = "hashicorp/precise64"
  config.vm.define "server" do |server|
    server.vm.hostname = "server"
    server.vm.synced_folder ENV["GOPATH"] + "/src/Bowery/crosswalk", "/home/vagrant/gocode/src/Bowery/crosswalk"
    server.vm.network "private_network", ip: "10.0.0.11"

    $script = <<-SCRIPT
      apt-get install mercurial -y
      SRCROOT="/opt/go"
      apt-get update
      hg clone -u release https://code.google.com/p/go ${SRCROOT}
      cd ${SRCROOT}/src
      ./all.bash
cat <<EOF >/tmp/gopath.sh
  export GOPATH="/home/vagrant/gocode"
  export PATH="/opt/go/bin:\$GOPATH/bin:\$PATH"
EOF
      mv /tmp/gopath.sh /etc/profile.d/gopath.sh
      chmod 0755 /etc/profile.d/gopath.sh
      sudo chmod 0755 /etc/profile.d/gopath.sh
      sudo chmod 777 -R /opt/go
    SCRIPT

    server.vm.provision "shell", inline: $script
  end
end
