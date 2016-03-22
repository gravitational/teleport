# this .bashrc is used as a profile script for "buildbox" container
[ -z "$PS1" ] && return
case "$TERM" in
    xterm-color) color_prompt=yes;;
esac

force_color_prompt=yes

if [ -x /usr/bin/dircolors ]; then
    test -r ~/.dircolors && eval "$(dircolors -b ~/.dircolors)" || eval "$(dircolors -b)"
    alias ls='ls --color=auto'
    alias grep='grep --color=auto'
    alias fgrep='fgrep --color=auto'
    alias egrep='egrep --color=auto'
fi

alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'

if [ -f /etc/bash_completion ] && ! shopt -oq posix; then
    . /etc/bash_completion
fi

shopt -s extglob

alias du='du --max-depth=1 -h'
alias ll="ls -lh"
alias la="ls -a"
alias df="df -hT"
export EDITOR=/usr/bin/vim

export PS1='\[\033[32;1m\]build-box\[\033[0;32m\] \w\[\033[00m\]: '
