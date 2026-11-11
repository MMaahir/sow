package project

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/MMaahir/sow/internal/answers"
	"github.com/MMaahir/sow/internal/render"
)

// pipeline is the work that sow new and sow update share. It brings the
// project in dir from the old render to the new render, writes the answers
// file and runs the follow-up steps. A new project has no old render, so
// every file of the new render is written, and the pipeline then makes the
// first commit.
type pipeline struct {
	// dir is the root of the project.
	dir string
	// environ is the environment for the commands the pipeline starts.
	environ []string
	// from is the release of the old render, and recorded holds the answers
	// it renders with. from is "" for a new project, which has no old render.
	from     string
	recorded answers.Answers
	// to is the release of template, and wanted holds the answers the new
	// render uses. The answers file records both.
	to       string
	wanted   answers.Answers
	template fs.FS
	// repo is the git repo of sow that the old template is fetched from, and
	// cacheDir holds the templates fetched before and the merge inputs.
	repo     string
	cacheDir string
	// github puts a new project on GitHub after the first commit.
	github bool
}

// isNew reports whether the pipeline creates the project.
func (p pipeline) isNew() bool {
	return p.from == ""
}

// run checks, renders, writes or merges every file, writes the answers file
// and runs the follow-up steps. A new project then gets its first commit, and
// with p.github goes on GitHub.
//
// It returns the paths that git merge-file left conflict markers in, sorted. When there are any, it skips the follow-up
// steps, because a manifest with conflict markers can't be installed. A failed
// check, fetch or render leaves the disk unchanged. A failure while it changes
// files, or a failed follow-up step, leaves the files it changed so far.
func (p pipeline) run(ctx context.Context) ([]string, error) {
	project := newTools(p.environ, p.dir)
	remote, err := p.preflight(ctx, project)
	if err != nil {
		return nil, err
	}
	oldRender, newRender, err := p.render(ctx, project)
	if err != nil {
		return nil, err
	}
	answersFile, err := answers.Marshal(p.to, p.wanted)
	if err != nil {
		return nil, err
	}
	if err := checkNoSymlinks(p.dir, oldRender, newRender); err != nil {
		return nil, err
	}
	changes, err := planChanges(p.dir, oldRender, newRender)
	if err != nil {
		return nil, err
	}

	conflicts, err := p.applyChanges(ctx, project, changes)
	if err != nil && !p.isNew() {
		// Files written before the failure stay changed, so say how to undo them.
		return nil, fmt.Errorf("%w\nsow update stopped partway. Run git reset --hard && git clean -fd "+
			"in %s to undo its changes", err, p.dir)
	}
	if err != nil {
		return nil, err
	}
	// The merged files are at release p.to even when some have conflicts.
	if err := render.Write(p.dir, []render.File{{Path: answers.FileName, Content: answersFile}}); err != nil {
		return nil, fmt.Errorf("write the answers file: %w", err)
	}
	if len(conflicts) > 0 {
		return conflicts, nil
	}
	steps, err := followUpSteps(p.dir, p.isNew())
	if err != nil {
		return nil, err
	}
	if err := runSteps(ctx, project, steps); err != nil {
		return nil, err
	}
	if !p.isNew() {
		return nil, nil
	}
	return nil, p.commit(ctx, project, remote)
}

// preflight runs the checks that read the machine before anything is
// written. A new project's directory must not exist, and its checks run in
// the directory above it. An updated project must be a clean git repo. With
// p.github, gh must be ready, and preflight returns the URL of the GitHub
// repo. When the wanted answers turn on a TypeScript component, Vite+ must be
// ready.
func (p pipeline) preflight(ctx context.Context, project *tools) (string, error) {
	checks := project
	if p.isNew() {
		if err := CheckTarget(p.dir); err != nil {
			return "", err
		}
		checks = project.in(filepath.Dir(p.dir))
	} else if err := checkClean(ctx, project); err != nil {
		return "", err
	}
	var remote string
	if p.github {
		var err error
		remote, err = checkGitHub(ctx, checks, p.wanted["github_user"], p.wanted[answers.ProjectName])
		if err != nil {
			return "", err
		}
	}
	if p.wanted.TypeScriptOn() {
		if err := checkViteplus(ctx, checks); err != nil {
			return "", err
		}
	}
	return remote, nil
}

// render returns the old render and the new render. The old render is nil for
// a new project. Otherwise it fetches the template of release p.from from
// p.repo, unless p.cacheDir already has it.
func (p pipeline) render(ctx context.Context, project *tools) ([]render.File, []render.File, error) {
	// An answers file keeps no entry for a component it lacks, but a render
	// needs one.
	wanted := p.wanted.WithComponentsOff()
	if p.isNew() {
		newRender, err := render.Render(p.template, wanted.Dirs(), wanted)
		return nil, newRender, err
	}
	oldTemplate, err := p.fetchTemplate(ctx, project)
	if err != nil {
		return nil, nil, err
	}
	recorded := p.recorded.WithComponentsOff()
	oldRender, err := render.Render(os.DirFS(oldTemplate), recorded.Dirs(), recorded)
	if err != nil {
		return nil, nil, fmt.Errorf("sow %s: %w", p.from, err)
	}
	newRender, err := render.Render(p.template, wanted.Dirs(), wanted)
	if err != nil {
		return nil, nil, err
	}
	return oldRender, newRender, nil
}

// commit makes the first commit of a new project and, with p.github, puts it
// on GitHub with origin at the URL remote. When a GitHub step fails, the
// project and its first commit stay on disk.
func (p pipeline) commit(ctx context.Context, project *tools, remote string) error {
	for _, args := range [][]string{
		{"init", "--initial-branch", "main"},
		{"add", "--all"},
		{"commit", "--message", "chore: initial commit from sow"},
	} {
		if err := project.run(ctx, "git", args...); err != nil {
			return err
		}
	}
	if !p.github {
		return nil
	}
	steps := githubSteps(p.wanted["github_user"], p.wanted[answers.ProjectName], remote)
	if err := runSteps(ctx, project, steps); err != nil {
		return fmt.Errorf("%w\nsow kept the project and its first commit in %s", err, p.dir)
	}
	return nil
}
