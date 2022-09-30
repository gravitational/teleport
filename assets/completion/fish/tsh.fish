complete -c tsh -e

function __query_complete
    set -l args $argv
    set -l cmdline (commandline -opc) (commandline -ct)
    set -e cmdline[1]

    # If no subcommand has been given, return so this can be used as a condition.
    test -n "$cmdline[1]"
    or return
    set -l cmd $cmdline[1]
    set -e cmdline[1]

    echo "$cmdline" | string match --regex "\-\-cluster[\ =]?(?<cluster>[a-z0-9]*)" > /dev/null

    switch "$cmd"
    case ls
        if test -n (echo "$cluster" | string trim)
            tsh ls --cluster $cluster | string match --regex "([a-z0-9]*=[a-z0-9]*)" | sort -h | uniq
            return
        end
        tsh ls | string match --regex "([a-z0-9]*=[a-z0-9]*)" | sort -h | uniq
    case ssh scp
        if test "$cmd" = "scp"
            __fish_complete_path 
        end
        if test -n (echo "$cluster" | string trim)
            tsh ls --cluster $cluster | string match --regex "^[a-z0-9\.]*" | sort -h | uniq
            return
        end
        tsh ls | string match --regex "^[a-z0-9\.]*" | sort -h | uniq
    case '*'
        return
    end
end

set -l global_commands login logout clusters \
                       status env config \
                       help version
set -l cluster_commands ssh scp apps db ls join play request kube mfa
complete --arguments "$global_commands $cluster_commands" \
         --command tsh \
         --no-files \
         --condition "not __fish_seen_subcommand_from $global_commands $cluster_commands"

set -l db_commands ls login logout env config connect
complete --arguments "$db_commands" \
         --command tsh \
         --no-files \
         --condition "__fish_seen_subcommand_from db && not __fish_seen_subcommand_from $db_commands"

set -l apps_commands ls login logout config
complete --arguments "$apps_commands" \
         --command tsh \
         --no-files \
         --condition "__fish_seen_subcommand_from apps && not __fish_seen_subcommand_from $apps_commands"

complete --long-option=cluster \
         --command tsh \
         --exclusive \
         --condition "not __fish_seen_subcommand_from $global_commands" \
         --argument "(tsh clusters | string match --regex '^[a-z0-9]*')"

complete --long-option debug \
         --short-option d \
         --command tsh \
         --no-files

complete --long-option option \
         --short-option o \
         --command tsh \
         --condition "__fish_seen_subcommand_from ssh" \
         --no-files

complete --command tsh \
         --arguments '(__query_complete)' \
         --condition "__fish_seen_subcommand_from ssh scp ls" \
         --no-files
