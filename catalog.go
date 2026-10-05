package libyaci

import (
	"sort"
	"strings"
	"sync"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// ChainInfo contains configured or detected metadata about the connected chain.
// Reflected capabilities remain the source of truth for method availability.
type ChainInfo struct {
	ChainID           string
	AppName           string
	AppVersion        string
	SDKVersion        string
	MinSDKVersion     string
	ReflectionVersion string
}

// ServiceInfo describes a reflected gRPC service.
type ServiceInfo struct {
	Name    string
	Methods []MethodInfo
}

// MethodInfo describes a reflected gRPC method.
type MethodInfo struct {
	FullName        string
	Service         string
	Name            string
	Input           string
	Output          string
	ClientStreaming bool
	ServerStreaming bool
}

// MessageInfo describes a reflected protobuf message.
type MessageInfo struct {
	FullName string
	File     string
}

// Catalog is an immutable snapshot of services, methods, and message types
// advertised by server reflection at client initialization time.
type Catalog struct {
	mu         sync.RWMutex
	chain      ChainInfo
	advertised map[string]bool
	services   map[string]ServiceInfo
	methods    map[string]MethodInfo
	messages   map[string]MessageInfo
}

func newCatalog(files *protoregistry.Files, advertisedServices []string, chain ChainInfo) *Catalog {
	c := &Catalog{
		chain:      chain,
		advertised: make(map[string]bool, len(advertisedServices)),
		services:   make(map[string]ServiceInfo),
		methods:    make(map[string]MethodInfo),
		messages:   make(map[string]MessageInfo),
	}

	for _, service := range advertisedServices {
		c.advertised[service] = true
	}

	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		collectMessages(c.messages, fd.Messages(), fd.Path())

		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			if !c.advertised[string(svc.FullName())] {
				continue
			}
			info := ServiceInfo{Name: string(svc.FullName())}
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				methodInfo := MethodInfo{
					FullName:        string(method.FullName()),
					Service:         string(svc.FullName()),
					Name:            string(method.Name()),
					Input:           string(method.Input().FullName()),
					Output:          string(method.Output().FullName()),
					ClientStreaming: method.IsStreamingClient(),
					ServerStreaming: method.IsStreamingServer(),
				}
				info.Methods = append(info.Methods, methodInfo)
				c.methods[methodInfo.FullName] = methodInfo
			}
			sort.Slice(info.Methods, func(i, j int) bool {
				return info.Methods[i].Name < info.Methods[j].Name
			})
			c.services[info.Name] = info
		}
		return true
	})

	return c
}

func collectMessages(dst map[string]MessageInfo, messages protoreflect.MessageDescriptors, file string) {
	for i := 0; i < messages.Len(); i++ {
		msg := messages.Get(i)
		dst[string(msg.FullName())] = MessageInfo{
			FullName: string(msg.FullName()),
			File:     file,
		}
		collectMessages(dst, msg.Messages(), file)
	}
}

// ChainInfo returns configured or detected chain metadata.
func (c *Catalog) ChainInfo() ChainInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.chain
}

func (c *Catalog) setChainInfo(info ChainInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chain = info
}

// Services returns all reflected services sorted by name.
func (c *Catalog) Services() []ServiceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	services := make([]ServiceInfo, 0, len(c.services))
	for _, svc := range c.services {
		svc.Methods = append([]MethodInfo(nil), svc.Methods...)
		services = append(services, svc)
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
	return services
}

// Methods returns all reflected methods sorted by full name.
func (c *Catalog) Methods() []MethodInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	methods := make([]MethodInfo, 0, len(c.methods))
	for _, method := range c.methods {
		methods = append(methods, method)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].FullName < methods[j].FullName
	})
	return methods
}

// Messages returns all reflected message types sorted by full name.
func (c *Catalog) Messages() []MessageInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	messages := make([]MessageInfo, 0, len(c.messages))
	for _, msg := range c.messages {
		messages = append(messages, msg)
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].FullName < messages[j].FullName
	})
	return messages
}

// QueryServices returns advertised services named Query, which is the standard
// Cosmos SDK module query-service convention.
func (c *Catalog) QueryServices() []ServiceInfo {
	var services []ServiceInfo
	for _, svc := range c.Services() {
		if strings.HasSuffix(svc.Name, ".Query") {
			services = append(services, svc)
		}
	}
	return services
}

// MessageTypesByPackage returns reflected messages in the requested proto
// package sorted by full name.
func (c *Catalog) MessageTypesByPackage(pkg string) []MessageInfo {
	prefix := pkg + "."
	var messages []MessageInfo
	for _, msg := range c.Messages() {
		if strings.HasPrefix(msg.FullName, prefix) && !strings.Contains(strings.TrimPrefix(msg.FullName, prefix), ".") {
			messages = append(messages, msg)
		}
	}
	return messages
}

// HasService reports whether the connected server exposes a service.
func (c *Catalog) HasService(service string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.services[service]
	return ok
}

// HasMethod reports whether the connected server exposes a method.
func (c *Catalog) HasMethod(method string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.methods[method]
	return ok
}

// Method returns metadata for a reflected method.
func (c *Catalog) Method(method string) (MethodInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.methods[method]
	return info, ok
}

// Service returns metadata for a reflected service.
func (c *Catalog) Service(service string) (ServiceInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.services[service]
	if !ok {
		return ServiceInfo{}, false
	}
	info.Methods = append([]MethodInfo(nil), info.Methods...)
	return info, true
}

func (c *Catalog) unsupportedMethod(method string) *UnsupportedMethodError {
	service, _, _ := parseMethodFullName(method)
	err := &UnsupportedMethodError{
		Method:     method,
		Service:    service,
		SDKVersion: c.ChainInfo().SDKVersion,
	}
	if service == "" {
		err.Reason = "method name must use package.Service.Method format"
		return err
	}
	c.mu.RLock()
	svc, ok := c.services[service]
	c.mu.RUnlock()
	if ok {
		err.Reason = "service is present but the method is not advertised by reflection"
		err.Available = make([]string, 0, len(svc.Methods))
		for _, method := range svc.Methods {
			err.Available = append(err.Available, method.Name)
		}
		return err
	}
	err.Reason = "service is not advertised by reflection"
	suffix := service[strings.LastIndex(service, ".")+1:]
	if suffix != service {
		for _, svc := range c.Services() {
			if strings.HasSuffix(svc.Name, "."+suffix) {
				err.AvailableServices = append(err.AvailableServices, svc.Name)
			}
		}
	}
	return err
}

// Catalog returns the client's reflected capability catalog.
func (c *Client) Catalog() *Catalog {
	return c.catalog
}

// ChainInfo returns configured or detected chain metadata.
func (c *Client) ChainInfo() ChainInfo {
	if c.catalog == nil {
		return ChainInfo{}
	}
	return c.catalog.ChainInfo()
}

// SupportsService reports whether the connected server exposes a service.
func (c *Client) SupportsService(service string) bool {
	return c.catalog != nil && c.catalog.HasService(service)
}

// SupportsMethod reports whether the connected server exposes a method.
func (c *Client) SupportsMethod(method string) bool {
	return c.catalog != nil && c.catalog.HasMethod(method)
}
