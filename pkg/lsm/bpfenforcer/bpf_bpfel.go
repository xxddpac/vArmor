// Code generated by bpf2go; DO NOT EDIT.
//go:build 386 || amd64 || amd64p32 || arm || arm64 || mips64le || mips64p32le || mipsle || ppc64le || riscv64
// +build 386 amd64 amd64p32 arm arm64 mips64le mips64p32le mipsle ppc64le riscv64

package bpfenforcer

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfBuffer struct{ Value [12288]uint8 }

// loadBpf returns the embedded CollectionSpec for bpf.
func loadBpf() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_BpfBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load bpf: %w", err)
	}

	return spec, err
}

// loadBpfObjects loads bpf and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*bpfObjects
//	*bpfPrograms
//	*bpfMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func loadBpfObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadBpf()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// bpfSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfSpecs struct {
	bpfProgramSpecs
	bpfMapSpecs
}

// bpfSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfProgramSpecs struct {
	VarmorBprmCheckSecurity *ebpf.ProgramSpec `ebpf:"varmor_bprm_check_security"`
	VarmorCapable           *ebpf.ProgramSpec `ebpf:"varmor_capable"`
	VarmorFileOpen          *ebpf.ProgramSpec `ebpf:"varmor_file_open"`
	VarmorPathLink          *ebpf.ProgramSpec `ebpf:"varmor_path_link"`
	VarmorPathLinkTail      *ebpf.ProgramSpec `ebpf:"varmor_path_link_tail"`
	VarmorPathRename        *ebpf.ProgramSpec `ebpf:"varmor_path_rename"`
	VarmorPathRenameTail    *ebpf.ProgramSpec `ebpf:"varmor_path_rename_tail"`
	VarmorPathSymlink       *ebpf.ProgramSpec `ebpf:"varmor_path_symlink"`
	VarmorSocketConnect     *ebpf.ProgramSpec `ebpf:"varmor_socket_connect"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	FileProgs    *ebpf.MapSpec `ebpf:"file_progs"`
	V_bprmOuter  *ebpf.MapSpec `ebpf:"v_bprm_outer"`
	V_capable    *ebpf.MapSpec `ebpf:"v_capable"`
	V_fileBuffer *ebpf.MapSpec `ebpf:"v_file_buffer"`
	V_fileOuter  *ebpf.MapSpec `ebpf:"v_file_outer"`
	V_netOuter   *ebpf.MapSpec `ebpf:"v_net_outer"`
}

// bpfObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfObjects struct {
	bpfPrograms
	bpfMaps
}

func (o *bpfObjects) Close() error {
	return _BpfClose(
		&o.bpfPrograms,
		&o.bpfMaps,
	)
}

// bpfMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfMaps struct {
	FileProgs    *ebpf.Map `ebpf:"file_progs"`
	V_bprmOuter  *ebpf.Map `ebpf:"v_bprm_outer"`
	V_capable    *ebpf.Map `ebpf:"v_capable"`
	V_fileBuffer *ebpf.Map `ebpf:"v_file_buffer"`
	V_fileOuter  *ebpf.Map `ebpf:"v_file_outer"`
	V_netOuter   *ebpf.Map `ebpf:"v_net_outer"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.FileProgs,
		m.V_bprmOuter,
		m.V_capable,
		m.V_fileBuffer,
		m.V_fileOuter,
		m.V_netOuter,
	)
}

// bpfPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfPrograms struct {
	VarmorBprmCheckSecurity *ebpf.Program `ebpf:"varmor_bprm_check_security"`
	VarmorCapable           *ebpf.Program `ebpf:"varmor_capable"`
	VarmorFileOpen          *ebpf.Program `ebpf:"varmor_file_open"`
	VarmorPathLink          *ebpf.Program `ebpf:"varmor_path_link"`
	VarmorPathLinkTail      *ebpf.Program `ebpf:"varmor_path_link_tail"`
	VarmorPathRename        *ebpf.Program `ebpf:"varmor_path_rename"`
	VarmorPathRenameTail    *ebpf.Program `ebpf:"varmor_path_rename_tail"`
	VarmorPathSymlink       *ebpf.Program `ebpf:"varmor_path_symlink"`
	VarmorSocketConnect     *ebpf.Program `ebpf:"varmor_socket_connect"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.VarmorBprmCheckSecurity,
		p.VarmorCapable,
		p.VarmorFileOpen,
		p.VarmorPathLink,
		p.VarmorPathLinkTail,
		p.VarmorPathRename,
		p.VarmorPathRenameTail,
		p.VarmorPathSymlink,
		p.VarmorSocketConnect,
	)
}

func _BpfClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed bpf_bpfel.o
var _BpfBytes []byte