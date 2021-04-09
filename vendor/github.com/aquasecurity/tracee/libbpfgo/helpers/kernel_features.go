package helpers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// These constants are a limited number of the total kernel config options,
// but are provided for convenience because they are most relevant for BPF
// development.
const (
	CONFIG_BPF                      = "CONFIG_BPF"
	CONFIG_BPF_SYSCALL              = "CONFIG_BPF_SYSCALL"
	CONFIG_HAVE_EBPF_JIT            = "CONFIG_HAVE_EBPF_JIT"
	CONFIG_BPF_JIT                  = "CONFIG_BPF_JIT"
	CONFIG_BPF_JIT_ALWAYS_ON        = "CONFIG_BPF_JIT_ALWAYS_ON"
	CONFIG_CGROUPS                  = "CONFIG_CGROUPS"
	CONFIG_CGROUP_BPF               = "CONFIG_CGROUP_BPF"
	CONFIG_CGROUP_NET_CLASSID       = "CONFIG_CGROUP_NET_CLASSID"
	CONFIG_SOCK_CGROUP_DATA         = "CONFIG_SOCK_CGROUP_DATA"
	CONFIG_BPF_EVENTS               = "CONFIG_BPF_EVENTS"
	CONFIG_KPROBE_EVENTS            = "CONFIG_KPROBE_EVENTS"
	CONFIG_UPROBE_EVENTS            = "CONFIG_UPROBE_EVENTS"
	CONFIG_TRACING                  = "CONFIG_TRACING"
	CONFIG_FTRACE_SYSCALLS          = "CONFIG_FTRACE_SYSCALLS"
	CONFIG_FUNCTION_ERROR_INJECTION = "CONFIG_FUNCTION_ERROR_INJECTION"
	CONFIG_BPF_KPROBE_OVERRIDE      = "CONFIG_BPF_KPROBE_OVERRIDE"
	CONFIG_NET                      = "CONFIG_NET"
	CONFIG_XDP_SOCKETS              = "CONFIG_XDP_SOCKETS"
	CONFIG_LWTUNNEL_BPF             = "CONFIG_LWTUNNEL_BPF"
	CONFIG_NET_ACT_BPF              = "CONFIG_NET_ACT_BPF"
	CONFIG_NET_CLS_BPF              = "CONFIG_NET_CLS_BPF"
	CONFIG_NET_CLS_ACT              = "CONFIG_NET_CLS_ACT"
	CONFIG_NET_SCH_INGRESS          = "CONFIG_NET_SCH_INGRESS"
	CONFIG_XFRM                     = "CONFIG_XFRM"
	CONFIG_IP_ROUTE_CLASSID         = "CONFIG_IP_ROUTE_CLASSID"
	CONFIG_IPV6_SEG6_BPF            = "CONFIG_IPV6_SEG6_BPF"
	CONFIG_BPF_LIRC_MODE2           = "CONFIG_BPF_LIRC_MODE2"
	CONFIG_BPF_STREAM_PARSER        = "CONFIG_BPF_STREAM_PARSER"
	CONFIG_NETFILTER_XT_MATCH_BPF   = "CONFIG_NETFILTER_XT_MATCH_BPF"
	CONFIG_BPFILTER                 = "CONFIG_BPFILTER"
	CONFIG_BPFILTER_UMH             = "CONFIG_BPFILTER_UMH"
	CONFIG_TEST_BPF                 = "CONFIG_TEST_BPF"
	CONFIG_HZ                       = "CONFIG_HZ"
	CONFIG_DEBUG_INFO_BTF           = "CONFIG_DEBUG_INFO_BTF"
	CONFIG_DEBUG_INFO_BTF_MODULES   = "CONFIG_DEBUG_INFO_BTF_MODULES"
)

type KernelConfig map[string]string

// InitKernelConfig populates the passed KernelConfig
// by attempting to read the kernel config into it from:
// /proc/config-$(uname -r)
// or
// /boot/config.gz
func InitKernelConfig(k KernelConfig) (KernelConfig, error) {

	x := unix.Utsname{}
	err := unix.Uname(&x)
	if err != nil {
		return nil, fmt.Errorf("could not determine uname release: %v", err)
	}

	bootConfigPath := fmt.Sprintf("/boot/config-%s", bytes.Trim(x.Release[:], "\x00"))

	err = k.getBootConfigByPath(bootConfigPath)
	if err == nil {
		return k, nil
	}

	err2 := k.getProcGZConfigByPath("/proc/config.gz")
	if err != nil {
		return nil, fmt.Errorf("%v %v", err, err2)
	}

	return k, nil
}

// GetKernelConfigValue retrieves a value from the kernel config
// If the config value does not exist an error will be returned
func (k KernelConfig) GetKernelConfigValue(key string) (string, error) {
	v, exists := k[key]
	if !exists {
		return "", errors.New("kernel config value does not exist, it's possible this option is not present in your kernel version or the KernelConfig has not been initialized")
	}
	return v, nil
}

func (k KernelConfig) getBootConfigByPath(bootConfigPath string) error {

	configFile, err := os.Open(bootConfigPath)
	if err != nil {
		return fmt.Errorf("could not open %s: %v", bootConfigPath, err)
	}

	k.readConfigFromScanner(configFile)

	return nil
}

func (k KernelConfig) getProcGZConfigByPath(procConfigPath string) error {

	configFile, err := os.Open(procConfigPath)
	if err != nil {
		return fmt.Errorf("could not open %s: %v", procConfigPath, err)
	}

	return k.getProcGZConfig(configFile)
}

func (k KernelConfig) getProcGZConfig(reader io.Reader) error {
	zreader, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}

	k.readConfigFromScanner(zreader)
	return nil
}

func (k KernelConfig) readConfigFromScanner(reader io.Reader) {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		kv := strings.Split(scanner.Text(), "=")
		if len(kv) != 2 {
			continue
		}

		k[kv[0]] = kv[1]
	}
}
