package main

const (
	// loginUsageFooter is printed at the bottom of `tsh help login` output
	loginUsageFooter = `NOTES:
  The proxy address format is host:https_port,ssh_proxy_port

EXAMPLES:
  Use ports 8080 and 8023 for https and SSH proxy:
  $ tsh --proxy=host.example.com:8080,8023 login    
  
  Use port 8080 and 3023 (default) for SSH proxy:
  $ tsh --proxy=host.example.com:8080 login`
)
