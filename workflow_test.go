package anchora_test

import (
	"context"
	"errors"
	"github.com/sagnikc395/anchora"
	"testing"
	"time"
)

type fakeAgent struct{ calls, failures int }

func (a *fakeAgent) Run(_ context.Context, prompt string) (string, error) {
	a.calls++
	if a.calls <= a.failures {
		return "", errors.New("temporary failure")
	}
	return "done: " + prompt, nil
}

type echoAgent struct{}

func (echoAgent) Run(_ context.Context, prompt string) (string, error) { return prompt, nil }
func TestWorkflowRunsDAGAndPassesOutputs(t *testing.T) {
	w, err := anchora.NewWorkflow([]anchora.Step{
		{ID: "research", Agent: echoAgent{}, Prompt: "facts"},
		{ID: "summary", Agent: echoAgent{}, Prompt: "summarize {{steps.research.output}}", DependsOn: []string{"research"}},
	}, anchora.Options{})
	if err != nil {
		t.Fatal(err)
	}
	results, err := w.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if results[1].Output != "summarize facts" {
		t.Fatalf("unexpected results: %#v", results)
	}
}
func TestWorkflowRejectsCycles(t *testing.T) {
	_, err := anchora.NewWorkflow([]anchora.Step{{ID: "a", Agent: echoAgent{}, Prompt: "a", DependsOn: []string{"b"}}, {ID: "b", Agent: echoAgent{}, Prompt: "b", DependsOn: []string{"a"}}}, anchora.Options{})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}
func TestWorkflowRetriesAndSucceeds(t *testing.T) {
	agent := &fakeAgent{failures: 2}
	w, err := anchora.NewWorkflow([]anchora.Step{{ID: "one", Agent: agent, Prompt: "work"}}, anchora.Options{MaxRetries: 2, RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	results, err := w.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if agent.calls != 3 || results[0].Attempts != 2 || results[0].Status != anchora.Succeeded {
		t.Fatalf("unexpected result: %#v, calls=%d", results[0], agent.calls)
	}
}

func TestWorkflowResumeSkipsCompletedSteps(t *testing.T) {
	agent := &fakeAgent{}
	w, err := anchora.NewWorkflow([]anchora.Step{
		{ID: "research", Agent: agent, Prompt: "facts"},
		{ID: "summary", Agent: agent, Prompt: "summarize {{steps.research.output}}", DependsOn: []string{"research"}},
	}, anchora.Options{Resume: []anchora.StepResult{
		{ID: "research", Status: anchora.Succeeded, Output: "cached facts"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	results, err := w.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if agent.calls != 1 {
		t.Fatalf("agent calls = %d, want 1 (the resumed step must not re-run)", agent.calls)
	}
	if results[0].Output != "cached facts" {
		t.Fatalf("resumed result was not replayed: %#v", results[0])
	}
	if results[1].Output != "done: summarize cached facts" {
		t.Fatalf("downstream step did not see the resumed output: %#v", results[1])
	}
}

func TestWorkflowResumeIgnoresUnfinishedResults(t *testing.T) {
	agent := &fakeAgent{}
	w, err := anchora.NewWorkflow([]anchora.Step{{ID: "one", Agent: agent, Prompt: "work"}},
		anchora.Options{Resume: []anchora.StepResult{{ID: "one", Status: anchora.Failed, Error: "boom"}}})
	if err != nil {
		t.Fatal(err)
	}
	results, err := w.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if agent.calls != 1 || results[0].Status != anchora.Succeeded {
		t.Fatalf("a failed prior result must be retried, got calls=%d result=%#v", agent.calls, results[0])
	}
}
