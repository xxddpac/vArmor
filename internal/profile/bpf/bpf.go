// Copyright 2023 vArmor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bpf

import (
	"fmt"
	"net"
	"strings"

	"github.com/dlclark/regexp2"
	"golang.org/x/sys/unix"

	varmor "github.com/bytedance/vArmor/apis/varmor/v1beta1"
	varmortypes "github.com/bytedance/vArmor/pkg/types"
)

const (
	PreciseMatch = 0x00000001
	GreedyMatch  = 0x00000002
	PrefixMatch  = 0x00000004
	SuffixMatch  = 0x00000008
	CidrMatch    = 0x00000020
	Ipv4Match    = 0x00000040
	Ipv6Match    = 0x00000080
	PortMatch    = 0x00000100
	AaMayExec    = 0x00000001
	AaMayWrite   = 0x00000002
	AaMayRead    = 0x00000004
	AaMayAppend  = 0x00000008
)

func reverseString(s string) string {
	bytes := []byte(s)
	len := len(bytes)

	for i := 0; i < len/2; i++ {
		bytes[i], bytes[len-i-1] = bytes[len-i-1], bytes[i]
	}

	return string(bytes)
}

func regexp2FindAllString(re *regexp2.Regexp, s string) []string {
	var matches []string
	m, _ := re.FindStringMatch(s)
	for m != nil {
		matches = append(matches, m.String())
		m, _ = re.FindNextMatch(m)
	}
	return matches
}

func GenerateRuntimeDefaultProfile(bpfContent *varmor.BpfContent) error {
	var err error

	fileContent, err := newBpfPathRule("/proc/sysrq-trigger", AaMayRead|AaMayWrite|AaMayAppend)
	if err != nil {
		return err
	}
	bpfContent.Files = append(bpfContent.Files, *fileContent)

	fileContent, err = newBpfPathRule("/proc/**/mem", AaMayRead|AaMayWrite|AaMayAppend)
	if err != nil {
		return err
	}
	bpfContent.Files = append(bpfContent.Files, *fileContent)

	fileContent, err = newBpfPathRule("/proc/kmem", AaMayRead|AaMayWrite|AaMayAppend)
	if err != nil {
		return err
	}
	bpfContent.Files = append(bpfContent.Files, *fileContent)

	fileContent, err = newBpfPathRule("/proc/kcore", AaMayRead|AaMayWrite|AaMayAppend)
	if err != nil {
		return err
	}
	bpfContent.Files = append(bpfContent.Files, *fileContent)

	fileContent, err = newBpfPathRule("/sys/firmware/**", AaMayRead|AaMayWrite|AaMayAppend)
	if err != nil {
		return err
	}
	bpfContent.Files = append(bpfContent.Files, *fileContent)

	fileContent, err = newBpfPathRule("/sys/kernel/security/**", AaMayRead|AaMayWrite|AaMayAppend)
	if err != nil {
		return err
	}
	bpfContent.Files = append(bpfContent.Files, *fileContent)

	return nil
}

func newBpfPathRule(pattern string, permissions uint32) (*varmor.FileContent, error) {
	// Pre-check
	re, err := regexp2.Compile(`(?<!\*)\*(?!\*)`, regexp2.None)
	if err != nil {
		return nil, err
	}
	starWildcardLen := len(regexp2FindAllString(re, pattern))

	if starWildcardLen > 0 && strings.Contains(pattern, "**") {
		return nil, fmt.Errorf("the globbing * and ** in the pattern cannot be used at the same time")
	}

	if starWildcardLen > 1 || strings.Count(pattern, "**") > 1 {
		return nil, fmt.Errorf("the globbing * or ** in the pattern can only be used once")
	}

	// Create bpfPathRule
	var pathRule varmor.FileContent
	var flags uint32

	if starWildcardLen > 0 {
		if strings.Contains(pattern, "/") {
			return nil, fmt.Errorf("the pattern with globbing * is not supported")
		}

		stringList := strings.Split(pattern, "*")

		if len(stringList[0]) > 0 {
			pathRule.Prefix = stringList[0]
			flags |= PrefixMatch
		}

		if len(stringList[1]) > 0 {
			pathRule.Suffix = reverseString(stringList[1])
			flags |= SuffixMatch
		}
	} else if strings.Contains(pattern, "**") {
		flags |= GreedyMatch

		stringList := strings.Split(pattern, "**")

		if len(stringList[0]) > 0 {
			pathRule.Prefix = stringList[0]
			flags |= PrefixMatch
		}

		if len(stringList[1]) > 0 {
			pathRule.Suffix = reverseString(stringList[1])
			flags |= SuffixMatch
		}
	} else {
		pathRule.Prefix = pattern
		flags |= PreciseMatch | PrefixMatch
	}

	pathRule.Flags = flags
	pathRule.Permissions = permissions

	return &pathRule, nil
}

func newBpfNetworkRule(cidr string, ipAddress string, port uint32) (*varmor.NetworkContent, error) {
	// Pre-check
	if cidr == "" && ipAddress == "" && port == 0 {
		return nil, fmt.Errorf("cidr, ipAddress and port cannot be empty at the same time")
	}

	if cidr != "" && ipAddress != "" {
		return nil, fmt.Errorf("cannot set CIRD and IP address at the same time")
	}

	if port > 65535 {
		return nil, fmt.Errorf("invalid network port")
	}

	var networkRule varmor.NetworkContent

	if cidr != "" {
		networkRule.Flags |= CidrMatch

		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}

		networkRule.Address = ipNet.IP.String()
		networkRule.CIDR = ipNet.String()
		if ipNet.IP.To4() != nil {
			networkRule.Flags |= Ipv4Match
		} else {
			networkRule.Flags |= Ipv6Match
		}
	}

	if ipAddress != "" {
		networkRule.Flags |= PreciseMatch

		ip := net.ParseIP(ipAddress)
		if ip == nil {
			return nil, fmt.Errorf("the address is not a valid textual representation of an IP address")
		}

		networkRule.Address = ip.String()
		if ip.To4() != nil {
			networkRule.Flags |= Ipv4Match
		} else {
			networkRule.Flags |= Ipv6Match
		}
	}

	if port != 0 {
		networkRule.Flags |= PortMatch
		networkRule.Port = port
	}

	return &networkRule, nil
}

func generateHardeningRules(rule string, content *varmor.BpfContent) error {
	rule = strings.ToLower(rule)
	rule = strings.ReplaceAll(rule, "_", "-")

	switch rule {
	//// 1. Blocking escape vectors from privileged container
	// disallow write core_pattern
	case "disallow-write-core-pattern":
		fileContent, err := newBpfPathRule("/proc/sys/kernel/core_pattern", AaMayWrite|AaMayAppend)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)
	// disallow write release_agent
	case "disallow-write-release-agent":
		fileContent, err := newBpfPathRule("/sys/fs/cgroup/**/release_agent", AaMayWrite|AaMayAppend)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)
	// disallow insmond
	case "disallow-insmod":
		content.Capabilities |= 1 << unix.CAP_SYS_MODULE
	// disallow load ebpf program
	case "disallow-load-ebpf":
		content.Capabilities |= (1 << unix.CAP_SYS_ADMIN) | (1 << unix.CAP_BPF)

	//// 2. Disable capabilities
	// disable all capabilities
	case "disable-cap-all":
		content.Capabilities = (1 << (unix.CAP_LAST_CAP + 1)) - 1
	// disable privileged capabilities
	case "disable-cap-privileged":
		content.Capabilities |= ((1 << unix.CAP_DAC_READ_SEARCH) |
			(1 << unix.CAP_LINUX_IMMUTABLE) |
			(1 << unix.CAP_NET_BROADCAST) |
			(1 << unix.CAP_NET_ADMIN) |
			(1 << unix.CAP_IPC_LOCK) |
			(1 << unix.CAP_IPC_OWNER) |
			(1 << unix.CAP_SYS_MODULE) |
			(1 << unix.CAP_SYS_RAWIO) |
			(1 << unix.CAP_SYS_PTRACE) |
			(1 << unix.CAP_SYS_PACCT) |
			(1 << unix.CAP_SYS_ADMIN) |
			(1 << unix.CAP_SYS_BOOT) |
			(1 << unix.CAP_SYS_NICE) |
			(1 << unix.CAP_SYS_RESOURCE) |
			(1 << unix.CAP_SYS_TIME) |
			(1 << unix.CAP_SYS_TTY_CONFIG) |
			(1 << unix.CAP_LEASE) |
			(1 << unix.CAP_AUDIT_CONTROL) |
			(1 << unix.CAP_MAC_OVERRIDE) |
			(1 << unix.CAP_MAC_ADMIN) |
			(1 << unix.CAP_SYSLOG) |
			(1 << unix.CAP_WAKE_ALARM) |
			(1 << unix.CAP_BLOCK_SUSPEND) |
			(1 << unix.CAP_AUDIT_READ))
	// disable the specified capability
	case "disable-cap-chown":
		content.Capabilities |= 1 << unix.CAP_CHOWN
	case "disable-cap-dac-override":
		content.Capabilities |= 1 << unix.CAP_DAC_OVERRIDE
	case "disable-cap-dac-read-search":
		content.Capabilities |= 1 << unix.CAP_DAC_READ_SEARCH
	case "disable-cap-fowner":
		content.Capabilities |= 1 << unix.CAP_FOWNER
	case "disable-cap-fsetid":
		content.Capabilities |= 1 << unix.CAP_FSETID
	case "disable-cap-kill":
		content.Capabilities |= 1 << unix.CAP_KILL
	case "disable-cap-setgid":
		content.Capabilities |= 1 << unix.CAP_SETGID
	case "disable-cap-setuid":
		content.Capabilities |= 1 << unix.CAP_SETUID
	case "disable-cap-setpcap":
		content.Capabilities |= 1 << unix.CAP_SETPCAP
	case "disable-cap-linux-immutable":
		content.Capabilities |= 1 << unix.CAP_LINUX_IMMUTABLE
	case "disable-cap-net-bind-service":
		content.Capabilities |= 1 << unix.CAP_NET_BIND_SERVICE
	case "disable-cap-net-broadcast":
		content.Capabilities |= 1 << unix.CAP_NET_BROADCAST
	case "disable-cap-net-admin":
		content.Capabilities |= 1 << unix.CAP_NET_ADMIN
	case "disable-cap-net-raw":
		content.Capabilities |= 1 << unix.CAP_NET_RAW
	case "disable-cap-ipc-lock":
		content.Capabilities |= 1 << unix.CAP_IPC_LOCK
	case "disable-cap-ipc-owner":
		content.Capabilities |= 1 << unix.CAP_IPC_OWNER
	case "disable-cap-sys-module":
		content.Capabilities |= 1 << unix.CAP_SYS_MODULE
	case "disable-cap-sys-rawio":
		content.Capabilities |= 1 << unix.CAP_SYS_RAWIO
	case "disable-cap-sys-chroot":
		content.Capabilities |= 1 << unix.CAP_SYS_CHROOT
	case "disable-cap-sys-ptrace":
		content.Capabilities |= 1 << unix.CAP_SYS_PTRACE
	case "disable-cap-sys-pacct":
		content.Capabilities |= 1 << unix.CAP_SYS_PACCT
	case "disable-cap-sys-admin":
		content.Capabilities |= 1 << unix.CAP_SYS_ADMIN
	case "disable-cap-sys-boot":
		content.Capabilities |= 1 << unix.CAP_SYS_BOOT
	case "disable-cap-sys-nice":
		content.Capabilities |= 1 << unix.CAP_SYS_NICE
	case "disable-cap-sys-resource":
		content.Capabilities |= 1 << unix.CAP_SYS_RESOURCE
	case "disable-cap-sys-time":
		content.Capabilities |= 1 << unix.CAP_SYS_TIME
	case "disable-cap-sys-tty-config":
		content.Capabilities |= 1 << unix.CAP_SYS_TTY_CONFIG
	case "disable-cap-mknod":
		content.Capabilities |= 1 << unix.CAP_MKNOD
	case "disable-cap-lease":
		content.Capabilities |= 1 << unix.CAP_LEASE
	case "disable-cap-audit-write":
		content.Capabilities |= 1 << unix.CAP_AUDIT_WRITE
	case "disable-cap-audit-control":
		content.Capabilities |= 1 << unix.CAP_AUDIT_CONTROL
	case "disable-cap-setfcap":
		content.Capabilities |= 1 << unix.CAP_SETFCAP
	case "disable-cap-mac-override":
		content.Capabilities |= 1 << unix.CAP_MAC_OVERRIDE
	case "disable-cap-mac-admin":
		content.Capabilities |= 1 << unix.CAP_MAC_ADMIN
	case "disable-cap-syslog":
		content.Capabilities |= 1 << unix.CAP_SYSLOG
	case "disable-cap-wake-alarm":
		content.Capabilities |= 1 << unix.CAP_WAKE_ALARM
	case "disable-cap-block-suspend":
		content.Capabilities |= 1 << unix.CAP_BLOCK_SUSPEND
	case "disable-cap-audit-read":
		content.Capabilities |= 1 << unix.CAP_AUDIT_READ
	case "disable-cap-perfmon":
		content.Capabilities |= 1 << unix.CAP_PERFMON
	case "disable-cap-bpf":
		content.Capabilities |= 1 << unix.CAP_BPF

	//// 3. Kernel vulnerability mitigation
	// diallow create user namespace
	case "disallow-create-user-ns":
		// TODO: add support for userns_create hook point (Linux v6.1+)
		fallthrough
	// diallow abuse user namespace
	case "disallow-abuse-user-ns":
		content.Capabilities |= 1 << unix.CAP_SYS_ADMIN
	}
	return nil
}

func generateVulMitigationRules(rule string, content *varmor.BpfContent) error {
	rule = strings.ToLower(rule)
	rule = strings.ReplaceAll(rule, "_", "-")

	switch rule {
	case "cgroups-lxcfs-escape-mitigation":
		fileContent, err := newBpfPathRule("/**/release_agent", AaMayWrite|AaMayAppend)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)

		fileContent, err = newBpfPathRule("/**/devices.allow", AaMayWrite|AaMayAppend)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)

		fileContent, err = newBpfPathRule("/**/cgroup.procs", AaMayWrite|AaMayAppend)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)

		fileContent, err = newBpfPathRule("/**/devices/tasks", AaMayWrite|AaMayAppend)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)
	}
	return nil
}

func generateAttackProtectionRules(rule string, content *varmor.BpfContent) error {
	var fileContent *varmor.FileContent
	var networkContent *varmor.NetworkContent
	var err error

	rule = strings.ToLower(rule)
	rule = strings.ReplaceAll(rule, "_", "-")

	switch rule {
	//// 4. Mitigate container information leakage
	case "mitigate-sa-leak":
		fileContent, err = newBpfPathRule("/run/secrets/kubernetes.io/serviceaccount/**", AaMayRead)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)

		fileContent, err = newBpfPathRule("/var/run/secrets/kubernetes.io/serviceaccount/**", AaMayRead)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)
	case "mitigate-disk-device-number-leak":
		fileContent, err = newBpfPathRule("/proc/partitions", AaMayRead)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)

		fileContent, err = newBpfPathRule("/proc/**/mountinfo", AaMayRead)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)
	case "mitigate-overlayfs-leak":
		fileContent, err = newBpfPathRule("/proc/**/mounts", AaMayRead)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)

		fileContent, err = newBpfPathRule("/proc/**/mountinfo", AaMayRead)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)
	case "mitigate-host-ip-leak":
		fileContent, err = newBpfPathRule("/proc/**/net/arp", AaMayRead)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)
	case "disallow-metadata-service":
		// For Aliyun, Volc Engine, etc.
		networkContent, err = newBpfNetworkRule("", "100.96.0.96", 0)
		if err != nil {
			return err
		}
		content.Networks = append(content.Networks, *networkContent)

		// For AWS, GCP, Azure, etc.
		networkContent, err = newBpfNetworkRule("", "169.254.169.254", 0)
		if err != nil {
			return err
		}
		content.Networks = append(content.Networks, *networkContent)

	//// 5. Restrict the sensitive operations inside the container
	case "disable-write-etc":
		fileContent, err = newBpfPathRule("/etc/**", AaMayWrite|AaMayAppend)
		if err != nil {
			return err
		}
		content.Files = append(content.Files, *fileContent)

	case "disable-busybox":
		fileContent, err = newBpfPathRule("/**/busybox", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)

	case "disable-shell":
		fileContent, err = newBpfPathRule("/**/sh", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)

		fileContent, err = newBpfPathRule("/**/bash", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)

		fileContent, err = newBpfPathRule("/**/dash", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)
	case "disable-wget":
		fileContent, err = newBpfPathRule("/**/wget", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)
	case "disable-curl":
		fileContent, err = newBpfPathRule("/**/curl", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)
	case "disable-chmod":
		fileContent, err = newBpfPathRule("/**/chmod", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)
	case "disable-su-sudo":
		fileContent, err = newBpfPathRule("/**/su", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)

		fileContent, err = newBpfPathRule("/**/sudo", AaMayExec)
		if err != nil {
			return err
		}
		content.Processes = append(content.Processes, *fileContent)
	}
	return nil
}

func generateRawFileRules(rule varmor.FileRule, bpfContent *varmor.BpfContent) error {
	var permissions uint32

	for _, permission := range rule.Permissions {
		switch strings.ToLower(permission) {
		case "read", "r":
			permissions |= AaMayRead
		case "write", "w":
			permissions |= AaMayWrite
			permissions |= AaMayAppend
		case "append", "a":
			permissions |= AaMayAppend
		}
	}

	if permissions == 0 {
		return nil
	}

	fileContent, err := newBpfPathRule(rule.Pattern, permissions)
	if err != nil {
		return err
	}
	bpfContent.Files = append(bpfContent.Files, *fileContent)

	return nil
}

func generateRawProcessRules(rule varmor.FileRule, bpfContent *varmor.BpfContent) error {
	var permissions uint32

	for _, permission := range rule.Permissions {
		switch strings.ToLower(permission) {
		case "exec", "x":
			permissions |= AaMayExec
		}
	}

	if permissions == 0 {
		return nil
	}

	fileContent, err := newBpfPathRule(rule.Pattern, permissions)
	if err != nil {
		return err
	}
	bpfContent.Processes = append(bpfContent.Processes, *fileContent)

	return nil
}

func generateRawNetworkRules(rule varmor.NetworkEgressRule, bpfContent *varmor.BpfContent) error {
	networkContent, err := newBpfNetworkRule(rule.IPBlock, rule.IP, uint32(rule.Port))
	if err != nil {
		return err
	}
	bpfContent.Networks = append(bpfContent.Networks, *networkContent)

	return nil
}

func GenerateEnhanceProtectProfile(enhanceProtect *varmor.EnhanceProtect, bpfContent *varmor.BpfContent) error {
	var err error
	// Hardening
	for _, rule := range enhanceProtect.HardeningRules {
		err = generateHardeningRules(rule, bpfContent)
		if err != nil {
			return err
		}
	}

	// Vulnerability Mitigation
	for _, rule := range enhanceProtect.VulMitigationRules {
		err = generateVulMitigationRules(rule, bpfContent)
		if err != nil {
			return err
		}
	}

	// Attack Protection
	for _, attackProtectionRule := range enhanceProtect.AttackProtectionRules {
		if len(attackProtectionRule.Targets) == 0 {
			for _, rule := range attackProtectionRule.Rules {
				err = generateAttackProtectionRules(rule, bpfContent)
				if err != nil {
					return err
				}
			}
		}
	}

	// Custom
	for _, rule := range enhanceProtect.BpfRawRules.Files {
		err := generateRawFileRules(rule, bpfContent)
		if err != nil {
			return err
		}

		err = generateRawProcessRules(rule, bpfContent)
		if err != nil {
			return err
		}
	}

	for _, rule := range enhanceProtect.BpfRawRules.Processes {
		err := generateRawFileRules(rule, bpfContent)
		if err != nil {
			return err
		}

		err = generateRawProcessRules(rule, bpfContent)
		if err != nil {
			return err
		}
	}

	for _, egressRule := range enhanceProtect.BpfRawRules.Network.Egresses {
		err := generateRawNetworkRules(egressRule, bpfContent)
		if err != nil {
			return err
		}
	}

	if len(bpfContent.Files) > varmortypes.MaxBpfFileRuleCount {
		return fmt.Errorf("the maximum number of BPF file rules exceeded(Max Count: %d)", varmortypes.MaxBpfFileRuleCount)
	}

	if len(bpfContent.Processes) > varmortypes.MaxBpfBprmRuleCount {
		return fmt.Errorf("the maximum number of BPF bprm rules exceeded(Max Count: %d)", varmortypes.MaxBpfBprmRuleCount)
	}

	if len(bpfContent.Networks) > varmortypes.MaxBpfNetworkRuleCount {
		return fmt.Errorf("the maximum number of BPF network rules exceeded(Max Count: %d)", varmortypes.MaxBpfNetworkRuleCount)
	}

	return nil
}
