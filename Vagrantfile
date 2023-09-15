# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrantfile is used for BPF testing. See also bpf/README.md.
#
# To build Teleport, try to build as much as you can outside of Vagrant and
# "stealing" the necessary build incantations from `make build/teleport`.
# `make bpf-bytecode` is also a good starting point.

Vagrant.configure("2") do |config|
  # Kernel >=5.8 is required for BPF.
  #
  # Ideally we would use ubuntu/groovy64 (20.10), as it ships with kernel 5.8,
  # but sadly the box is not available anymore.
  #
  # * https://askubuntu.com/questions/517136/list-of-ubuntu-versions-with-corresponding-linux-kernel-version
  # * https://app.vagrantup.com/boxes/search
  config.vm.box = "ubuntu/jammy64"

  # Buff the VM. YMMV.
  config.vm.provider "virtualbox" do |v|
    v.cpus = 8
    v.memory = 8192
  end

  # Notable packages:
  #
  # * clang:               required to build BPF
  # * go:                  1.21.x (Snap, any version >= 1.21 works)
  # * libbpf-dev:          0.5.0  (not installed)
  # * libelf, zlib:        required by libbpf
  # * linux-tools-generic: bpftool
  # * llvm:                llvm-strip
  config.vm.provision "shell", name: "packages", inline: <<-SHELL
    apt-get update
    apt-get install -y --no-install-recommends \
      build-essential \
      ca-certificates \
      clang \
      libelf-dev \
      "linux-tools-$(uname -r)" \
      linux-tools-generic \
      llvm \
      pkg-config \
      zlib1g-dev

    snap install go --classic
    go version
  SHELL

  # Clone, build and install libbpf.
  #
  # Needs to be installed at "/usr/libbpf-$VERSION" for Teleport to be happy.
  # Eg, "/usr/libbpf-1.0.1".
  config.vm.provision "shell", name: "libbpf", privileged: false, inline: <<-SHELL
    LIBBPF_VERSION="$(grep LIBBPF_VERSION /vagrant/build.assets/versions.mk | awk '{print $3}')"
    DEST="/usr/libbpf-$LIBBPF_VERSION"
    echo "LIBBPF_VERSION = $LIBBPF_VERSION" # eg, 1.0.1

    cd ~
    [[ ! -d libbpf ]] && git clone --depth 1 https://github.com/libbpf/libbpf.git -b "v$LIBBPF_VERSION"
    cd libbpf/src
    make
    sudo make install DESTDIR="/opt/libbpf"
    sudo rm -fr "$DEST"
    sudo mv /opt/libbpf/usr "$DEST" && sudo rm -fr /opt/libbpf
  SHELL

  # Start sessions at /vagrant.
  config.vm.provision "shell", name: "user", privileged: false, inline: <<-SHELL
    if ! grep -q 'cd /vagrant' ~/.bashrc; then
      echo -e '\ncd /vagrant' >> ~/.bashrc
    fi
  SHELL

  # Forward proxy port.
  config.vm.network "forwarded_port", guest: 3080, host: 5080
end
