# common routines shared by Vagrantfiles for different providers
DOCKER_VER ||= "1.10.3"

NODES ||= {
  "a-auth" => ["a-auth", "10.0.10.10", "auth-a", "cluster-a"],
  "b-auth" => ["b-auth", "10.0.10.20", "auth-b", "cluster-b"], 
  "web-1"  => ["a-node", "10.0.10.12"],
  "web-2"  => ["a-node", "10.0.10.13"],
  "redis"  => ["a-node", "10.0.10.14"],
  "postgres-1" => ["a-node", "10.0.10.15"],
  "postgres-2" => ["a-node", "10.0.10.16"],
}


def configure_teleport(vm)
  vm.provision "file", source: '../teleport.service', destination: '/tmp/teleport.service'
  vm.provision "shell", inline: <<-SHELL
    cp -f /tmp/teleport.service /etc/systemd/system/
    systemctl daemon-reload
    systemctl enable teleport.service
    systemctl start teleport.service
  SHELL
end


def install_docker(vm, docker_version)
  vm.provision "file", source: '../docker.service', destination: '/tmp/docker.service'
  vm.provision "file", source: '../docker.socket', destination: '/tmp/docker.socket'

  vm.provision "shell", inline: <<-SHELL
    echo "Installing Docker..."
    groupadd docker
    gpasswd -a vagrant docker
    ls /tmp/docker*
    mv /tmp/docker* /etc/systemd/system/
    if [ ! -s /usr/bin/docker ]; then
        echo "Downloading Docker #{docker_version}..."
        wget -qO /usr/bin/docker https://get.docker.com/builds/Linux/x86_64/docker-#{docker_version} 
        chmod +x /usr/bin/docker
    fi
    systemctl daemon-reload
    systemctl enable docker.socket
    systemctl enable docker.service
    echo "Starting Docker..."
    systemctl restart docker
  SHELL
end


# this updates all apt packages (especially important for VirtualBox guest addition packages)
def apt_update(vm)
  vm.provision "shell", inline: <<-SHELL
    if [ ! -f /root/apt.updated ]; then
        apt-get -y update
        apt-get -y purge exim4-* libcairo*
        apt-get -y autoremove
        apt-get -y upgrade
        apt-get -y dist-upgrade
        apt-get -y install htop tree vim aufs-tools screen curl
        touch /root/apt.updated
    fi
  SHELL
end



# basic/recommended configuration of every machine:
def basic_config(vm)
  hosts = NODES.map { |hostname, array| "#{array[1]} #{hostname} #{array[1,5].join(" ")}" }.join("\n")
  bashrc="/home/vagrant/.bashrc"
  vm.provision "shell", inline: <<-SHELL
    if ! grep -q "git-core" #{bashrc} ; then 
        echo "customizing ~/bashrc"
        echo "\n\n# Customizations from Vagrantfile:" >> #{bashrc}
        echo "export PS1='\\[\\033[31;1m\\]\\h\\[\\033[0;32m\\] \\w\\[\\033[00m\\]: '" >> #{bashrc}
        echo export PATH="\$PATH:/usr/lib/git-core:/home/vagrant/teleport/build" >> #{bashrc}
        echo export GREP_OPTIONS="--color=auto" >> #{bashrc}
        echo "alias ll='ls -lh'" >> #{bashrc}
        echo "alias tsh='tsh --insecure'" >> #{bashrc}
    fi
    if ! grep -q "Teleport" /etc/hosts ; then 
        echo "# Teleport entries added by Vagrant:" >> /etc/hosts
        echo -e "#{hosts}" >> /etc/hosts
    fi
    mkdir -p /var/lib/teleport
    chown vagrant:vagrant /var/lib/teleport
  SHELL
end


# re-creates clean ~/.ssh on a VM, populated with your (host) ssh credentials
def configure_ssh(vm)
  vm.provision "shell", inline: <<-SHELL
    mkdir -p /home/vagrant/.ssh
    rm -rf /home/vagrant/.ssh/id_rsa*
    chown vagrant:vagrant /home/vagrant/.ssh
  SHELL
  vm.provision "file", source: '~/.ssh/id_rsa', destination: '~/.ssh/id_rsa'
  vm.provision "file", source: '~/.ssh/id_rsa.pub', destination: '~/.ssh/id_rsa.pub'
  vm.provision "file", source: '~/.screenrc', destination: '~/' if File.exists? "~/.screnrc"
end
