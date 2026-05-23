// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"fmt"
	"strings"

	"maand/bucket"
	"maand/workspace"
)

type jobCatalog struct {
	manifests map[string]workspace.Manifest
	versions  map[string]workspace.Version
	commands  map[string]map[string]struct{}
}

func loadJobCatalog(jobWorkspace *workspace.DefaultWorkspace, jobNames []string) (*jobCatalog, error) {
	catalog := &jobCatalog{
		manifests: make(map[string]workspace.Manifest, len(jobNames)),
		versions:  make(map[string]workspace.Version, len(jobNames)),
		commands:  make(map[string]map[string]struct{}, len(jobNames)),
	}

	for _, jobName := range jobNames {
		manifest, err := jobWorkspace.GetJobManifest(jobName)
		if err != nil {
			return nil, err
		}
		catalog.manifests[jobName] = manifest

		commandSet := make(map[string]struct{})
		for _, command := range manifest.ListedCommands() {
			commandSet[command.Name] = struct{}{}
		}
		catalog.commands[jobName] = commandSet
	}

	for _, jobName := range jobNames {
		version, err := catalog.resolveJobVersion(jobName, catalog.manifests[jobName])
		if err != nil {
			return nil, err
		}
		catalog.versions[jobName] = version
	}

	return catalog, nil
}

func (c *jobCatalog) resolveJobVersion(jobName string, manifest workspace.Manifest) (workspace.Version, error) {
	raw := strings.TrimSpace(manifest.Version)
	needsExplicit := manifest.RequiresExplicitVersion() || c.isDemandTarget(jobName)

	if raw == "" {
		if needsExplicit {
			return workspace.Version{}, fmt.Errorf("%w: job %q must declare version (dependency participant)",
				bucket.ErrInvalidJobVersion, jobName)
		}
		return workspace.Version{}, nil
	}

	version, err := workspace.ParseVersion(raw)
	if err != nil {
		return workspace.Version{}, fmt.Errorf("job %q: %w", jobName, err)
	}
	return version, nil
}

func (c *jobCatalog) isDemandTarget(jobName string) bool {
	for _, manifest := range c.manifests {
		for _, command := range manifest.ListedCommands() {
			if strings.TrimSpace(command.Demands.Job) == jobName {
				return true
			}
		}
	}
	return false
}

// ValidateJobCommandDemands checks demand references and version constraints across the workspace.
func ValidateJobCommandDemands(jobWorkspace *workspace.DefaultWorkspace, jobNames []string) error {
	jobNames = sortedJobNames(jobNames)
	catalog, err := loadJobCatalog(jobWorkspace, jobNames)
	if err != nil {
		return err
	}

	for _, jobName := range jobNames {
		manifest := catalog.manifests[jobName]
		for _, command := range manifest.ListedCommands() {
			if err := workspace.ValidateDemandReference(jobName, command.Name, command); err != nil {
				return err
			}

			demandJob := strings.TrimSpace(command.Demands.Job)
			if demandJob == "" {
				continue
			}

			if _, ok := catalog.manifests[demandJob]; !ok {
				return fmt.Errorf("%w: job %s command %s demands unknown job %q",
					bucket.ErrInvalidJobCommandDemand, jobName, command.Name, demandJob)
			}

			demandCommand := strings.TrimSpace(command.Demands.Command)
			if _, ok := catalog.commands[demandJob][demandCommand]; !ok {
				return fmt.Errorf("%w: job %s command %s demands job %q command %q which is not declared",
					bucket.ErrInvalidJobCommandDemand, jobName, command.Name, demandJob, demandCommand)
			}

			constraint, err := workspace.ParseVersionConstraint(command.Demands.Config)
			if err != nil {
				return fmt.Errorf("job %s command %s: %w", jobName, command.Name, err)
			}
			if constraint.Min == nil && constraint.Max == nil {
				continue
			}

			upstreamVersion := catalog.versions[demandJob]
			if strings.TrimSpace(catalog.manifests[demandJob].Version) == "" {
				return fmt.Errorf("%w: job %q must declare version (required by %s command %s)",
					bucket.ErrInvalidJobVersion, demandJob, jobName, command.Name)
			}
			if err := constraint.Satisfies(upstreamVersion); err != nil {
				return fmt.Errorf("job %s command %s depends on %s: %w",
					jobName, command.Name, demandJob, err)
			}
		}
	}

	return nil
}