package bucket

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidWorkerJSON                = errors.New("invalid worker.json")
	ErrInvalidManifest                  = errors.New("invalid manifest.json")
	ErrInvalidJobVars                   = errors.New("invalid vars.toml")
	ErrInvalidMaandConf                 = errors.New("invalid maand.conf")
	ErrInvalidBucketConf                = errors.New("invalid bucket.conf")
	ErrUnexpectedError                  = errors.New("unexpected error")
	ErrKeyNotFound                      = errors.New("key not found")
	ErrNotInitialized                   = errors.New("maand is not initialized")
	ErrDatabase                         = errors.New("database error")
	ErrNotFound                         = errors.New("not found")
	ErrPortCollision                    = errors.New("port collision")
	ErrPortKeyFormat                    = errors.New("invalid port key format")
	ErrInvalidPortRange                 = errors.New("invalid port range in bucket.conf")
	ErrPortRangeExhausted               = errors.New("port range exhausted")
	ErrInvalidManifestPort                = errors.New("invalid manifest port declaration")
	ErrInvalidJobCommandConfiguration   = errors.New("invalid job command configuration")
	ErrInvalidJobCommandDemand          = errors.New("invalid job command demand")
	ErrJobCommandDemandVersionMismatch  = errors.New("job command demand version mismatch")
	ErrInvalidJobVersion                = errors.New("invalid job version")
	ErrCircularJobCommandDependency     = errors.New("circular job command dependency")
	ErrJobCommandFileNotFound           = errors.New("job command file not found")
	ErrInvalidJob                       = errors.New("invalid job")
	ErrInsufficientResource             = errors.New("insufficient resource")
	ErrHealthCheckFailed                = errors.New("health check failed")
	ErrUnsupportedResourceConfiguration = errors.New("unsupported resource configuration")
	ErrRunCommand                       = errors.New("run command failed")
	ErrJobCommandFailed                 = errors.New("job command failed")
	ErrWorkerPrerequisites              = errors.New("worker prerequisites not met")
	ErrHostPrerequisites                = errors.New("host prerequisites not met")
)

// Deprecated: use ErrInvalidWorkerJSON.
var ErrInvaildWorkerJSON = ErrInvalidWorkerJSON

// Deprecated: use ErrInsufficientResource.
var ErrInSufficientResource = ErrInsufficientResource

// Deprecated: use ErrUnsupportedResourceConfiguration.
var ErrUnsupportedResourceConfigration = ErrUnsupportedResourceConfiguration

func KeyNotFoundError(namespace, key string) error {
	return fmt.Errorf("%w: namespace %s, key %s", ErrKeyNotFound, namespace, key)
}

func DatabaseError(err error) error {
	return fmt.Errorf("%w: %w", ErrDatabase, err)
}

func UnexpectedError(err error) error {
	return fmt.Errorf("%w: %w", ErrUnexpectedError, err)
}

func NotFoundError(domain string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, domain)
}
