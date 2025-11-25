package bucket

import (
	"errors"
	"fmt"
)

var (
	ErrInvaildWorkerJSON               = errors.New("invaild worker.json")
	ErrInvalidManifest                 = errors.New("invalid manifest.json")
	ErrInvalidMaandConf                = errors.New("invalid maand.conf")
	ErrInvalidBucketConf               = errors.New("invalid bucket.conf")
	ErrUnexpectedError                 = errors.New("unexpected error")
	ErrKeyNotFound                     = errors.New("key not found")
	ErrNotInitialized                  = errors.New("maand is not initialized")
	ErrDatabase                        = errors.New("database error")
	ErrNotFound                        = errors.New("not found")
	ErrPortCollision                   = errors.New("port collision")
	ErrPortKeyFormat                   = errors.New("invaild port key format")
	ErrInvalidJobCommandConfiguration  = errors.New("invaild job command configuration")
	ErrJobCommandFileNotFound          = errors.New("job command file not found")
	ErrInvalidJob                      = errors.New("invaild job")
	ErrInSufficientResource            = errors.New("insufficient resource")
	ErrHealthCheckFailed               = errors.New("health failed")
	ErrUnsupportedResourceConfigration = errors.New("unsupport resource configuration")
)

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
