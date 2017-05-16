// This file was generated by counterfeiter
package volumefakes

import (
	"sync"
	"time"

	"github.com/concourse/baggageclaim/volume"
)

type FakeFilesystemInitVolume struct {
	HandleStub        func() string
	handleMutex       sync.RWMutex
	handleArgsForCall []struct{}
	handleReturns     struct {
		result1 string
	}
	DataPathStub        func() string
	dataPathMutex       sync.RWMutex
	dataPathArgsForCall []struct{}
	dataPathReturns     struct {
		result1 string
	}
	LoadPropertiesStub        func() (volume.Properties, error)
	loadPropertiesMutex       sync.RWMutex
	loadPropertiesArgsForCall []struct{}
	loadPropertiesReturns     struct {
		result1 volume.Properties
		result2 error
	}
	StorePropertiesStub        func(volume.Properties) error
	storePropertiesMutex       sync.RWMutex
	storePropertiesArgsForCall []struct {
		arg1 volume.Properties
	}
	storePropertiesReturns struct {
		result1 error
	}
	LoadTTLStub        func() (volume.TTL, time.Time, error)
	loadTTLMutex       sync.RWMutex
	loadTTLArgsForCall []struct{}
	loadTTLReturns     struct {
		result1 volume.TTL
		result2 time.Time
		result3 error
	}
	StoreTTLStub        func(volume.TTL) (time.Time, error)
	storeTTLMutex       sync.RWMutex
	storeTTLArgsForCall []struct {
		arg1 volume.TTL
	}
	storeTTLReturns struct {
		result1 time.Time
		result2 error
	}
	LoadPrivilegedStub        func() (bool, error)
	loadPrivilegedMutex       sync.RWMutex
	loadPrivilegedArgsForCall []struct{}
	loadPrivilegedReturns     struct {
		result1 bool
		result2 error
	}
	StorePrivilegedStub        func(bool) error
	storePrivilegedMutex       sync.RWMutex
	storePrivilegedArgsForCall []struct {
		arg1 bool
	}
	storePrivilegedReturns struct {
		result1 error
	}
	ParentStub        func() (volume.FilesystemLiveVolume, bool, error)
	parentMutex       sync.RWMutex
	parentArgsForCall []struct{}
	parentReturns     struct {
		result1 volume.FilesystemLiveVolume
		result2 bool
		result3 error
	}
	DestroyStub        func() error
	destroyMutex       sync.RWMutex
	destroyArgsForCall []struct{}
	destroyReturns     struct {
		result1 error
	}
	InitializeStub        func() (volume.FilesystemLiveVolume, error)
	initializeMutex       sync.RWMutex
	initializeArgsForCall []struct{}
	initializeReturns     struct {
		result1 volume.FilesystemLiveVolume
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeFilesystemInitVolume) Handle() string {
	fake.handleMutex.Lock()
	fake.handleArgsForCall = append(fake.handleArgsForCall, struct{}{})
	fake.recordInvocation("Handle", []interface{}{})
	fake.handleMutex.Unlock()
	if fake.HandleStub != nil {
		return fake.HandleStub()
	}
	return fake.handleReturns.result1
}

func (fake *FakeFilesystemInitVolume) HandleCallCount() int {
	fake.handleMutex.RLock()
	defer fake.handleMutex.RUnlock()
	return len(fake.handleArgsForCall)
}

func (fake *FakeFilesystemInitVolume) HandleReturns(result1 string) {
	fake.HandleStub = nil
	fake.handleReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeFilesystemInitVolume) DataPath() string {
	fake.dataPathMutex.Lock()
	fake.dataPathArgsForCall = append(fake.dataPathArgsForCall, struct{}{})
	fake.recordInvocation("DataPath", []interface{}{})
	fake.dataPathMutex.Unlock()
	if fake.DataPathStub != nil {
		return fake.DataPathStub()
	}
	return fake.dataPathReturns.result1
}

func (fake *FakeFilesystemInitVolume) DataPathCallCount() int {
	fake.dataPathMutex.RLock()
	defer fake.dataPathMutex.RUnlock()
	return len(fake.dataPathArgsForCall)
}

func (fake *FakeFilesystemInitVolume) DataPathReturns(result1 string) {
	fake.DataPathStub = nil
	fake.dataPathReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeFilesystemInitVolume) LoadProperties() (volume.Properties, error) {
	fake.loadPropertiesMutex.Lock()
	fake.loadPropertiesArgsForCall = append(fake.loadPropertiesArgsForCall, struct{}{})
	fake.recordInvocation("LoadProperties", []interface{}{})
	fake.loadPropertiesMutex.Unlock()
	if fake.LoadPropertiesStub != nil {
		return fake.LoadPropertiesStub()
	}
	return fake.loadPropertiesReturns.result1, fake.loadPropertiesReturns.result2
}

func (fake *FakeFilesystemInitVolume) LoadPropertiesCallCount() int {
	fake.loadPropertiesMutex.RLock()
	defer fake.loadPropertiesMutex.RUnlock()
	return len(fake.loadPropertiesArgsForCall)
}

func (fake *FakeFilesystemInitVolume) LoadPropertiesReturns(result1 volume.Properties, result2 error) {
	fake.LoadPropertiesStub = nil
	fake.loadPropertiesReturns = struct {
		result1 volume.Properties
		result2 error
	}{result1, result2}
}

func (fake *FakeFilesystemInitVolume) StoreProperties(arg1 volume.Properties) error {
	fake.storePropertiesMutex.Lock()
	fake.storePropertiesArgsForCall = append(fake.storePropertiesArgsForCall, struct {
		arg1 volume.Properties
	}{arg1})
	fake.recordInvocation("StoreProperties", []interface{}{arg1})
	fake.storePropertiesMutex.Unlock()
	if fake.StorePropertiesStub != nil {
		return fake.StorePropertiesStub(arg1)
	}
	return fake.storePropertiesReturns.result1
}

func (fake *FakeFilesystemInitVolume) StorePropertiesCallCount() int {
	fake.storePropertiesMutex.RLock()
	defer fake.storePropertiesMutex.RUnlock()
	return len(fake.storePropertiesArgsForCall)
}

func (fake *FakeFilesystemInitVolume) StorePropertiesArgsForCall(i int) volume.Properties {
	fake.storePropertiesMutex.RLock()
	defer fake.storePropertiesMutex.RUnlock()
	return fake.storePropertiesArgsForCall[i].arg1
}

func (fake *FakeFilesystemInitVolume) StorePropertiesReturns(result1 error) {
	fake.StorePropertiesStub = nil
	fake.storePropertiesReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeFilesystemInitVolume) LoadTTL() (volume.TTL, time.Time, error) {
	fake.loadTTLMutex.Lock()
	fake.loadTTLArgsForCall = append(fake.loadTTLArgsForCall, struct{}{})
	fake.recordInvocation("LoadTTL", []interface{}{})
	fake.loadTTLMutex.Unlock()
	if fake.LoadTTLStub != nil {
		return fake.LoadTTLStub()
	}
	return fake.loadTTLReturns.result1, fake.loadTTLReturns.result2, fake.loadTTLReturns.result3
}

func (fake *FakeFilesystemInitVolume) LoadTTLCallCount() int {
	fake.loadTTLMutex.RLock()
	defer fake.loadTTLMutex.RUnlock()
	return len(fake.loadTTLArgsForCall)
}

func (fake *FakeFilesystemInitVolume) LoadTTLReturns(result1 volume.TTL, result2 time.Time, result3 error) {
	fake.LoadTTLStub = nil
	fake.loadTTLReturns = struct {
		result1 volume.TTL
		result2 time.Time
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeFilesystemInitVolume) StoreTTL(arg1 volume.TTL) (time.Time, error) {
	fake.storeTTLMutex.Lock()
	fake.storeTTLArgsForCall = append(fake.storeTTLArgsForCall, struct {
		arg1 volume.TTL
	}{arg1})
	fake.recordInvocation("StoreTTL", []interface{}{arg1})
	fake.storeTTLMutex.Unlock()
	if fake.StoreTTLStub != nil {
		return fake.StoreTTLStub(arg1)
	}
	return fake.storeTTLReturns.result1, fake.storeTTLReturns.result2
}

func (fake *FakeFilesystemInitVolume) StoreTTLCallCount() int {
	fake.storeTTLMutex.RLock()
	defer fake.storeTTLMutex.RUnlock()
	return len(fake.storeTTLArgsForCall)
}

func (fake *FakeFilesystemInitVolume) StoreTTLArgsForCall(i int) volume.TTL {
	fake.storeTTLMutex.RLock()
	defer fake.storeTTLMutex.RUnlock()
	return fake.storeTTLArgsForCall[i].arg1
}

func (fake *FakeFilesystemInitVolume) StoreTTLReturns(result1 time.Time, result2 error) {
	fake.StoreTTLStub = nil
	fake.storeTTLReturns = struct {
		result1 time.Time
		result2 error
	}{result1, result2}
}

func (fake *FakeFilesystemInitVolume) LoadPrivileged() (bool, error) {
	fake.loadPrivilegedMutex.Lock()
	fake.loadPrivilegedArgsForCall = append(fake.loadPrivilegedArgsForCall, struct{}{})
	fake.recordInvocation("LoadPrivileged", []interface{}{})
	fake.loadPrivilegedMutex.Unlock()
	if fake.LoadPrivilegedStub != nil {
		return fake.LoadPrivilegedStub()
	}
	return fake.loadPrivilegedReturns.result1, fake.loadPrivilegedReturns.result2
}

func (fake *FakeFilesystemInitVolume) LoadPrivilegedCallCount() int {
	fake.loadPrivilegedMutex.RLock()
	defer fake.loadPrivilegedMutex.RUnlock()
	return len(fake.loadPrivilegedArgsForCall)
}

func (fake *FakeFilesystemInitVolume) LoadPrivilegedReturns(result1 bool, result2 error) {
	fake.LoadPrivilegedStub = nil
	fake.loadPrivilegedReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakeFilesystemInitVolume) StorePrivileged(arg1 bool) error {
	fake.storePrivilegedMutex.Lock()
	fake.storePrivilegedArgsForCall = append(fake.storePrivilegedArgsForCall, struct {
		arg1 bool
	}{arg1})
	fake.recordInvocation("StorePrivileged", []interface{}{arg1})
	fake.storePrivilegedMutex.Unlock()
	if fake.StorePrivilegedStub != nil {
		return fake.StorePrivilegedStub(arg1)
	}
	return fake.storePrivilegedReturns.result1
}

func (fake *FakeFilesystemInitVolume) StorePrivilegedCallCount() int {
	fake.storePrivilegedMutex.RLock()
	defer fake.storePrivilegedMutex.RUnlock()
	return len(fake.storePrivilegedArgsForCall)
}

func (fake *FakeFilesystemInitVolume) StorePrivilegedArgsForCall(i int) bool {
	fake.storePrivilegedMutex.RLock()
	defer fake.storePrivilegedMutex.RUnlock()
	return fake.storePrivilegedArgsForCall[i].arg1
}

func (fake *FakeFilesystemInitVolume) StorePrivilegedReturns(result1 error) {
	fake.StorePrivilegedStub = nil
	fake.storePrivilegedReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeFilesystemInitVolume) Parent() (volume.FilesystemLiveVolume, bool, error) {
	fake.parentMutex.Lock()
	fake.parentArgsForCall = append(fake.parentArgsForCall, struct{}{})
	fake.recordInvocation("Parent", []interface{}{})
	fake.parentMutex.Unlock()
	if fake.ParentStub != nil {
		return fake.ParentStub()
	}
	return fake.parentReturns.result1, fake.parentReturns.result2, fake.parentReturns.result3
}

func (fake *FakeFilesystemInitVolume) ParentCallCount() int {
	fake.parentMutex.RLock()
	defer fake.parentMutex.RUnlock()
	return len(fake.parentArgsForCall)
}

func (fake *FakeFilesystemInitVolume) ParentReturns(result1 volume.FilesystemLiveVolume, result2 bool, result3 error) {
	fake.ParentStub = nil
	fake.parentReturns = struct {
		result1 volume.FilesystemLiveVolume
		result2 bool
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeFilesystemInitVolume) Destroy() error {
	fake.destroyMutex.Lock()
	fake.destroyArgsForCall = append(fake.destroyArgsForCall, struct{}{})
	fake.recordInvocation("Destroy", []interface{}{})
	fake.destroyMutex.Unlock()
	if fake.DestroyStub != nil {
		return fake.DestroyStub()
	}
	return fake.destroyReturns.result1
}

func (fake *FakeFilesystemInitVolume) DestroyCallCount() int {
	fake.destroyMutex.RLock()
	defer fake.destroyMutex.RUnlock()
	return len(fake.destroyArgsForCall)
}

func (fake *FakeFilesystemInitVolume) DestroyReturns(result1 error) {
	fake.DestroyStub = nil
	fake.destroyReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeFilesystemInitVolume) Initialize() (volume.FilesystemLiveVolume, error) {
	fake.initializeMutex.Lock()
	fake.initializeArgsForCall = append(fake.initializeArgsForCall, struct{}{})
	fake.recordInvocation("Initialize", []interface{}{})
	fake.initializeMutex.Unlock()
	if fake.InitializeStub != nil {
		return fake.InitializeStub()
	}
	return fake.initializeReturns.result1, fake.initializeReturns.result2
}

func (fake *FakeFilesystemInitVolume) InitializeCallCount() int {
	fake.initializeMutex.RLock()
	defer fake.initializeMutex.RUnlock()
	return len(fake.initializeArgsForCall)
}

func (fake *FakeFilesystemInitVolume) InitializeReturns(result1 volume.FilesystemLiveVolume, result2 error) {
	fake.InitializeStub = nil
	fake.initializeReturns = struct {
		result1 volume.FilesystemLiveVolume
		result2 error
	}{result1, result2}
}

func (fake *FakeFilesystemInitVolume) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.handleMutex.RLock()
	defer fake.handleMutex.RUnlock()
	fake.dataPathMutex.RLock()
	defer fake.dataPathMutex.RUnlock()
	fake.loadPropertiesMutex.RLock()
	defer fake.loadPropertiesMutex.RUnlock()
	fake.storePropertiesMutex.RLock()
	defer fake.storePropertiesMutex.RUnlock()
	fake.loadTTLMutex.RLock()
	defer fake.loadTTLMutex.RUnlock()
	fake.storeTTLMutex.RLock()
	defer fake.storeTTLMutex.RUnlock()
	fake.loadPrivilegedMutex.RLock()
	defer fake.loadPrivilegedMutex.RUnlock()
	fake.storePrivilegedMutex.RLock()
	defer fake.storePrivilegedMutex.RUnlock()
	fake.parentMutex.RLock()
	defer fake.parentMutex.RUnlock()
	fake.destroyMutex.RLock()
	defer fake.destroyMutex.RUnlock()
	fake.initializeMutex.RLock()
	defer fake.initializeMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeFilesystemInitVolume) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ volume.FilesystemInitVolume = new(FakeFilesystemInitVolume)
