package main

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
)

// job is one scheduled run. Position is its place in the full shuffle of both
// stages (or of the pilot), so a run keeps its slot whichever stage selects it.
type job struct {
	Position int
	Task     string
	Arm      int
	Index    int
	Pilot    bool
}

func (j job) id() string { return fmt.Sprintf("%s-arm%d-r%d", j.Task, j.Arm, j.Index) }

// shuffleJobs is a Fisher-Yates over PCG output with the modulo taken by hand:
// the standard library documents neither Shuffle's algorithm nor IntN's as
// stable, and this order is pre-registered. The stream is keyed by the seed and
// the engine's name (and a label, so the pilot has its own order).
func shuffleJobs(jobs []job, seed int64, engine, label string) []job {
	hasher := fnv.New64a()
	hasher.Write([]byte(engine + label))
	source := rand.NewPCG(uint64(seed), hasher.Sum64())
	for i := len(jobs) - 1; i > 0; i-- {
		j := int(source.Uint64() % uint64(i+1))
		jobs[i], jobs[j] = jobs[j], jobs[i]
	}
	for i := range jobs {
		jobs[i].Position = i
	}
	return jobs
}

// sizes are the registered run counts (PINS): arms 1 and 3 run nStage1 per
// task in stage 1 and nTotal in all; arm 2 runs nArm2, in stage 1 only.
type sizes struct {
	NStage1, NTotal, NArm2 int
}

func (s sizes) cell(arm int) int {
	if arm == 2 {
		return s.NArm2
	}
	return s.NTotal
}

// schedule is the full randomized order of every task, arm and run index of
// both stages for one engine. Tasks are always the four canonical ones (and the
// arms each has), whatever subset a run selects, so a subset keeps its slots.
func schedule(seed int64, engine string, armsOfTask func(string) []int, s sizes) []job {
	var jobs []job
	for _, task := range canonicalTasks {
		for _, arm := range armsOfTask(task) {
			for index := 0; index < s.cell(arm); index++ {
				jobs = append(jobs, job{Task: task, Arm: arm, Index: index})
			}
		}
	}
	return shuffleJobs(jobs, seed, engine, "")
}

// selectJobs keeps one stage's indices and the requested tasks and arms, in
// shuffle order. Stage 2 is arms 1 and 3 only.
func selectJobs(all []job, stage int, s sizes, tasks []string, arms []int) []job {
	var out []job
	for _, j := range all {
		lo, hi := 0, s.NStage1
		if j.Arm == 2 {
			hi = s.NArm2
			if stage == 2 {
				continue
			}
		} else if stage == 2 {
			lo, hi = s.NStage1, s.NTotal
		}
		if j.Index < lo || j.Index >= hi {
			continue
		}
		if tasks != nil && !contains(tasks, j.Task) {
			continue
		}
		if arms != nil && !containsInt(arms, j.Arm) {
			continue
		}
		out = append(out, j)
	}
	return out
}

// pilotSchedule is DESIGN § Pilot and cap: per task, n1 runs on arm 1 and n3
// on arm 3, shuffled with their own order. Pilot runs are excluded from every
// result and written to their own directory.
func pilotSchedule(seed int64, engine string, tasks []string, n1, n3 int) []job {
	var jobs []job
	for _, task := range canonicalTasks {
		for i := 0; i < n1; i++ {
			jobs = append(jobs, job{Task: task, Arm: 1, Index: i, Pilot: true})
		}
		for i := 0; i < n3; i++ {
			jobs = append(jobs, job{Task: task, Arm: 3, Index: i, Pilot: true})
		}
	}
	jobs = shuffleJobs(jobs, seed, engine, ":pilot")
	if tasks == nil {
		return jobs
	}
	var out []job
	for _, j := range jobs {
		if contains(tasks, j.Task) {
			out = append(out, j)
		}
	}
	return out
}
