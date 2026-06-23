package libyaci

import (
	"context"
	"encoding/json"
	"errors"
)

const methodGetNodeInfoFullName = "cosmos.base.tendermint.v1beta1.Service.GetNodeInfo"

// DetectChainInfo queries GetNodeInfo when available and records chain metadata
// for diagnostics. Missing GetNodeInfo returns partial configured metadata.
func (c *Client) DetectChainInfo(ctx context.Context) (ChainInfo, error) {
	info := c.ChainInfo()
	if !c.SupportsMethod(methodGetNodeInfoFullName) {
		return info, &UnsupportedMethodError{
			Method:     methodGetNodeInfoFullName,
			Service:    "cosmos.base.tendermint.v1beta1.Service",
			SDKVersion: info.SDKVersion,
			Reason:     "GetNodeInfo is not advertised by reflection",
		}
	}

	method, err := c.Method(methodGetNodeInfoFullName)
	if err != nil {
		return info, err
	}
	resp, err := method.CallMap(ctx, nil)
	if err != nil {
		return info, err
	}
	data, err := resp.JSON()
	if err != nil {
		return info, err
	}

	var parsed struct {
		DefaultNodeInfo struct {
			Network string `json:"network"`
		} `json:"defaultNodeInfo"`
		ApplicationVersion struct {
			Name             string `json:"name"`
			AppName          string `json:"appName"`
			Version          string `json:"version"`
			GoVersion        string `json:"goVersion"`
			CosmosSDKVersion string `json:"cosmosSdkVersion"`
			BuildDeps        []struct {
				Path    string `json:"path"`
				Version string `json:"version"`
			} `json:"buildDeps"`
		} `json:"applicationVersion"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return info, err
	}

	info.ChainID = parsed.DefaultNodeInfo.Network
	info.AppName = parsed.ApplicationVersion.AppName
	if info.AppName == "" {
		info.AppName = parsed.ApplicationVersion.Name
	}
	info.AppVersion = parsed.ApplicationVersion.Version
	if parsed.ApplicationVersion.CosmosSDKVersion != "" {
		info.SDKVersion = parsed.ApplicationVersion.CosmosSDKVersion
	} else if depVersion := cosmosSDKVersionFromDeps(parsed.ApplicationVersion.BuildDeps); depVersion != "" {
		info.SDKVersion = depVersion
	}
	if c.catalog != nil {
		c.catalog.setChainInfo(info)
	}
	return info, nil
}

func cosmosSDKVersionFromDeps(deps []struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}) string {
	for _, dep := range deps {
		if dep.Path == "github.com/cosmos/cosmos-sdk" {
			return dep.Version
		}
	}
	return ""
}

// IsUnsupportedMethod reports whether err indicates missing reflected method
// support on the connected server.
func IsUnsupportedMethod(err error) bool {
	return errors.Is(err, ErrUnsupportedMethod)
}
