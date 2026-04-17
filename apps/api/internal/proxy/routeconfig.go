package proxy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// BuildRouteConfig constructs a RouteConfig from a domain and its pre-loaded features.
func BuildRouteConfig(domain generated.Domain, containerName string, features []generated.DomainRouteFeature) RouteConfig {
	var port int32 = 8080
	if domain.ContainerPort.Valid {
		port = domain.ContainerPort.Int32
	}

	var routeFeatures []RouteFeature
	for _, f := range features {
		routeFeatures = append(routeFeatures, RouteFeature{
			Type:    f.FeatureType,
			Config:  json.RawMessage(f.Config),
			Enabled: f.Enabled,
		})
	}

	return RouteConfig{
		Hostname:       domain.Hostname,
		TargetURL:      fmt.Sprintf("http://%s:%d", containerName, port),
		TLS:            domain.SslEnabled,
		ForceHTTPS:     domain.ForceHttps,
		SSLMode:        domain.SslMode,
		SSLProvider:    domain.SslProvider.String,
		CertPath:       domain.CertPath.String,
		KeyPath:        domain.KeyPath.String,
		Features:       routeFeatures,
		AdvancedConfig: domain.AdvancedConfig,
	}
}

// BuildRouteConfigFromDB loads features from the database and constructs a RouteConfig.
func BuildRouteConfigFromDB(ctx context.Context, queries *generated.Queries, domain generated.Domain, containerName string) (RouteConfig, error) {
	dbFeatures, err := queries.ListRouteFeaturesByDomain(ctx, domain.ID)
	if err != nil {
		return RouteConfig{}, fmt.Errorf("list route features for %s: %w", domain.Hostname, err)
	}
	return BuildRouteConfig(domain, containerName, dbFeatures), nil
}
