// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package workerfacts probes workers over SSH and updates workspace/workers.json.
package workerfacts

import (
	"fmt"
	"strings"
	"sync"

	"maand/bucket"
	"maand/build"
	"maand/prereq"
	"maand/utils"
	"maand/worker"
	"maand/workspace"
)

// Options configures worker_facts execution.
type Options struct {
	WorkerCSV   string
	LabelCSV    string
	Concurrency int
	DryRun      bool
	RunBuild    bool
}

type remoteProbe func(workerIP string) (workspace.WorkerFacts, error)

// Execute probes workers and updates workers.json.
func Execute(opts Options) error {
	if opts.Concurrency < 1 {
		return errConcurrencyTooLow()
	}
	if opts.WorkerCSV != "" && opts.LabelCSV != "" {
		return errWorkersAndLabelsTogether()
	}

	workers, err := workspace.ReadWorkersFile()
	if err != nil {
		return err
	}

	targetWorkers, err := selectTargetWorkers(workers, opts.WorkerCSV, opts.LabelCSV)
	if err != nil {
		return err
	}
	if len(targetWorkers) == 0 {
		return errNoTargetWorkers()
	}

	targetHosts := make([]string, 0, len(targetWorkers))
	for _, w := range targetWorkers {
		targetHosts = append(targetHosts, strings.TrimSpace(w.Host))
	}

	if err := prereq.CheckLocalRunCommand(); err != nil {
		return err
	}

	conf, err := bucket.GetMaandConf()
	if err != nil {
		return err
	}
	if err := worker.CheckRunCommandPrerequisites(targetHosts, conf.UseSUDO); err != nil {
		return err
	}

	updates, err := probeWorkers(targetHosts, opts.Concurrency, probeWorker)
	if err != nil {
		return err
	}

	changes := previewChanges(targetWorkers, updates)
	printChanges(changes, opts.DryRun)

	if opts.DryRun {
		fmt.Println("dry run: workers.json not modified")
		return nil
	}

	if len(changes) == 0 {
		fmt.Println("workers.json already up to date")
		return nil
	}

	if _, err := workspace.ApplyWorkerFacts(updates); err != nil {
		return err
	}
	fmt.Printf("updated %d worker(s) in workers.json\n", len(changes))

	if opts.RunBuild {
		return build.Execute(build.Options{})
	}
	return nil
}

func selectTargetWorkers(workers []workspace.WorkerRecord, workerCSV, labelCSV string) ([]workspace.WorkerRecord, error) {
	workerFilter := parseCSVList(workerCSV)
	labelFilter := parseCSVList(labelCSV)

	switch {
	case len(workerFilter) > 0:
		known := make(map[string]workspace.WorkerRecord, len(workers))
		for _, w := range workers {
			host := strings.TrimSpace(w.Host)
			if host != "" {
				known[host] = w
			}
		}
		unknown := make([]string, 0)
		selected := make([]workspace.WorkerRecord, 0, len(workerFilter))
		for _, host := range utils.Unique(workerFilter) {
			w, ok := known[host]
			if !ok {
				unknown = append(unknown, host)
				continue
			}
			selected = append(selected, w)
		}
		if len(unknown) > 0 {
			return nil, errUnknownWorkers(unknown)
		}
		return selected, nil

	case len(labelFilter) > 0:
		return workspace.FilterWorkersByLabels(workers, labelFilter), nil

	default:
		return workers, nil
	}
}

func parseCSVList(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func probeWorkers(hosts []string, concurrency int, probe remoteProbe) (map[string]workspace.WorkerFacts, error) {
	hosts = utils.Unique(hosts)
	if len(hosts) == 0 {
		return nil, errNoTargetWorkers()
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		failures = make(map[string]error, len(hosts))
		updates  = make(map[string]workspace.WorkerFacts, len(hosts))
		sem      = make(chan struct{}, concurrency)
	)

	for _, host := range hosts {
		wg.Add(1)
		sem <- struct{}{}
		go func(workerIP string) {
			defer wg.Done()
			defer func() { <-sem }()

			facts, err := probe(workerIP)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures[workerIP] = err
				return
			}
			updates[workerIP] = facts
		}(host)
	}

	wg.Wait()
	if len(failures) > 0 {
		return nil, errProbeFailures(failures)
	}
	return updates, nil
}

func probeWorker(workerIP string) (workspace.WorkerFacts, error) {
	output, err := worker.RunRemoteScriptCombined(workerIP, strings.NewReader(probeScript))
	if err != nil {
		return workspace.WorkerFacts{}, err
	}

	parsed, err := parseProbeOutput(output)
	if err != nil {
		return workspace.WorkerFacts{}, fmt.Errorf("parse probe output: %w", err)
	}
	return toWorkerFacts(parsed), nil
}

func previewChanges(workers []workspace.WorkerRecord, updates map[string]workspace.WorkerFacts) []workspace.WorkerFactsChange {
	changes := make([]workspace.WorkerFactsChange, 0, len(updates))
	for _, w := range workers {
		host := strings.TrimSpace(w.Host)
		facts, ok := updates[host]
		if !ok {
			continue
		}

		change := workspace.WorkerFactsChange{
			Host:      host,
			OldMemory: w.Memory,
			NewMemory: workspace.FormatMemoryMB(facts.MemoryMB),
			OldCPU:    w.CPU,
			NewCPU:    workspace.FormatCPUMHz(facts.CPUMHz),
		}
		if !memoryChanged(change.OldMemory, change.NewMemory) &&
			!cpuChanged(change.OldCPU, change.NewCPU) {
			continue
		}
		changes = append(changes, change)
	}
	return changes
}

func memoryChanged(left, right string) bool {
	leftMB, leftErr := utils.ParseMemoryMB(left)
	rightMB, rightErr := utils.ParseMemoryMB(right)
	if leftErr == nil && rightErr == nil {
		return int(leftMB+0.5) != int(rightMB+0.5)
	}
	return strings.TrimSpace(left) != strings.TrimSpace(right)
}

func cpuChanged(left, right string) bool {
	leftMHz, leftErr := utils.ParseCPUMHz(left)
	rightMHz, rightErr := utils.ParseCPUMHz(right)
	if leftErr == nil && rightErr == nil {
		return int(leftMHz+0.5) != int(rightMHz+0.5)
	}
	return strings.TrimSpace(left) != strings.TrimSpace(right)
}

func printChanges(changes []workspace.WorkerFactsChange, dryRun bool) {
	if len(changes) == 0 {
		return
	}
	prefix := "update"
	if dryRun {
		prefix = "would update"
	}
	for _, change := range changes {
		fmt.Printf("%s %s memory %q -> %q cpu %q -> %q\n", prefix, change.Host, change.OldMemory, change.NewMemory, change.OldCPU, change.NewCPU)
	}
}
