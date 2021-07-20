export PS1='\[\033[33;1m\]container(\h)\[\033[0;33m\] \w\[\033[00m\]: '
export PATH=$PATH:/teleport/build 
export LS_COLORS="rs=0:di=01;34:ln=01;36:mh=00:pi=40;33:so=01;35:do=01;35:bd=40;33;01:cd=40;33;01:or=40;31;01:mi=00:su=37;41:sg=30;43:ca=30;41:tw=30;42:ow=34;42:st=37;44:ex=01;32:*.tar=01;31:*.tgz=01;31:*.arc=01;31:*.arj=01;31:*.taz=01;31:*.lha=01;31:*.lz4=01;31:*.lzh=01;31:*.lzma=01;31:*.tlz=01;31:*.txz=01;31:*.tzo=01;31:*.t7z=01;31:*.zip=01;31:*.z=01;31:*.Z=01;31:*.dz=01;31:*.gz=01;31:*.lrz=01;31:*.lz=01;31:*.lzo=01;31:*.xz=01;31:*.bz2=01;31:*.bz=01;31:*.tbz=01;31:*.tbz2=01;31:*.tz=01;31:*.deb=01;31:*.rpm=01;31:*.jar=01;31:*.war=01;31:*.ear=01;31:*.sar=01;31:*.rar=01;31:*.alz=01;31:*.ace=01;31:*.zoo=01;31:*.cpio=01;31:*.7z=01;31:*.rz=01;31:*.cab=01;31:*.jpg=01;35:*.jpeg=01;35:*.gif=01;35:*.bmp=01;35:*.pbm=01;35:*.pgm=01;35:*.ppm=01;35:*.tga=01;35:*.xbm=01;35:*.xpm=01;35:*.tif=01;35:*.tiff=01;35:*.png=01;35:*.svg=01;35:*.svgz=01;35:*.mng=01;35:*.pcx=01;35:*.mov=01;35:*.mpg=01;35:*.mpeg=01;35:*.m2v=01;35:*.mkv=01;35:*.webm=01;35:*.ogm=01;35:*.mp4=01;35:*.m4v=01;35:*.mp4v=01;35:*.vob=01;35:*.qt=01;35:*.nuv=01;35:*.wmv=01;35:*.asf=01;35:*.rm=01;35:*.rmvb=01;35:*.flc=01;35:*.avi=01;35:*.fli=01;35:*.flv=01;35:*.gl=01;35:*.dl=01;35:*.xcf=01;35:*.xwd=01;35:*.yuv=01;35:*.cgm=01;35:*.emf=01;35:*.ogv=01;35:*.ogx=01;35:*.aac=00;36:*.au=00;36:*.flac=00;36:*.m4a=00;36:*.mid=00;36:*.midi=00;36:*.mka=00;36:*.mp3=00;36:*.mpc=00;36:*.ogg=00;36:*.ra=00;36:*.wav=00;36:*.oga=00;36:*.opus=00;36:*.spx=00;36:*.xspf=00;36:"

alias ls="ls --color=auto"
alias ll="ls -alF"

# quick way to get into teleport repo dir
alias t="cd $HOME/go/src/github.com/gravitational/teleport"

# start SSH agent on demo terminal
SSH_ENV="$HOME/.ssh/agent-environment"

function start_agent {
    echo "Initializing new SSH agent..."
    mkdir -p $HOME/.ssh
    cp -f /mnt/shared/certs/teleport-known_hosts.pub /root/.ssh/known_hosts
    cp -f /etc/teleport.d/scripts/ssh.cfg /root/.ssh/config
    /usr/bin/ssh-agent | sed 's/^echo/#echo/' > "${SSH_ENV}"
    echo succeeded
    chmod 600 "${SSH_ENV}"
    . "${SSH_ENV}" > /dev/null
    cd /mnt/shared/certs && /usr/bin/ssh-add bot;
}


if [ "${HOSTNAME}" == "term" ]; then
    if [ -f "${SSH_ENV}" ]; then
        . "${SSH_ENV}" > /dev/null
        ps ${SSH_AGENT_PID} | grep ssh-agent$ > /dev/null || {
            start_agent;
        }
    else
        start_agent;
    fi

    # These aliases use identity file behind the scene
    alias tsh="/etc/teleport.d/scripts/tsh.alias"
    alias tctl="/etc/teleport.d/scripts/tctl.alias"

    echo "Welcome to Teleport Lab."
    echo ""
    echo "Access servers, databases and web apps in your cluster securely with Teleport."
    echo "Try a couple of commands to get started."
    echo ""
    echo "Teleport speaks SSH. You can SSH into it using OpenSSH:"
    echo ""
    echo "ssh root@luna.teleport"
    echo ""
    echo "Teleport is a bastion server for your OpenSSH hosts. SSH into OpenSSH server and record all commands:"
    echo ""
    echo "ssh root@mars.openssh.teleport"
    echo ""
    echo "Run ansible on Teleport nodes and OpenSSH servers:"
    echo ""
    echo "cd /etc/teleport.d/ansible && ansible all -m ping"
    echo ""
    echo "Try Teleport's client command: tsh. It's like SSH, but with superpowers."
    echo "Find all hosts matching label env=example and run hostname command:"
    echo ""
    echo "tsh ssh root@env=example hostname"
fi
