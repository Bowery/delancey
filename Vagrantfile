# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "hashicorp/precise64"
  config.vm.box_check_update = false
  config.vm.network "private_network", ip: "192.168.33.55"
  config.vm.synced_folder "agent", "/home/vagrant/gocode/src/github.com/Bowery/desktop/bowery/agent"
end
