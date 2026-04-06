Vagrant.configure("2") do |config|
  config.vm.box = "generic/ubuntu2204"
  config.vm.hostname = "dev"

  config.vm.provider "libvirt" do |lv|
    lv.memory = 4096
    lv.cpus = 4
  end
  
  config.vm.provision "file", source: "./version.env", destination: "/tmp/version.env"
  config.vm.provision "shell", path: "./scripts/dev/install-deps.sh"
  config.vm.provision "shell", path: "./scripts/dev/install-docker.sh"
  config.vm.provision "shell", path: "./scripts/dev/install-containerlab.sh"
  
end