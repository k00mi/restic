package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/restic/restic/internal/errors"
	"github.com/restic/restic/internal/restic"

	"github.com/spf13/cobra"
)

var cmdErase = &cobra.Command{
	Use:   "erase [flags] PATH",
	Short: "Remove a path from an existing snapshot",
	Long: `
blub
`,
	DisableAutoGenTag: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runErase(eraseOptions, globalOptions, args)
	},
}

func init() {
	cmdRoot.AddCommand(cmdErase)

	f := cmdErase.Flags()
	f.StringVar(&eraseOptions.Snapshot, "snapshot", "", "the snapshot")
}

type EraseOptions struct {
	Snapshot string
	Host     string
	Paths    []string
	Tags     restic.TagLists
}

var eraseOptions EraseOptions

func eraseFromTree(ctx context.Context, tree *restic.Tree, repo restic.Repository, prefix string, pathComponents []string) (restic.ID, error) {
	if tree == nil {
		return restic.ID{}, fmt.Errorf("called with a nil tree")
	}
	if repo == nil {
		return restic.ID{}, fmt.Errorf("called with a nil repository")
	}
	l := len(pathComponents)
	if l == 0 {
		return restic.ID{}, fmt.Errorf("empty path components")
	}
	item := filepath.Join(prefix, pathComponents[0])
	for i, node := range tree.Nodes {
		if node.Name == pathComponents[0] {
			switch {
			case l == 1:
				fmt.Printf("Erasing node %s\n", node.Name)
				tree.Nodes = append(tree.Nodes[:i], tree.Nodes[i+1:]...)

				treeID, err := repo.SaveTree(ctx, tree)
				if err != nil {
					return restic.ID{}, errors.Wrapf(err, "cannot store new tree")
				}
				fmt.Printf("Saved tree %q\n", treeID)

				return treeID, nil
			case l > 1 && node.Type == "dir":
				subtree, err := repo.LoadTree(ctx, *node.Subtree)
				if err != nil {
					return restic.ID{}, errors.Wrapf(err, "cannot load subtree for %q", item)
				}

				erasedSubtree, err := eraseFromTree(ctx, subtree, repo, item, pathComponents[1:])
				if err != nil {
					return restic.ID{}, err
				}

				node.Subtree = &erasedSubtree

				treeID, err := repo.SaveTree(ctx, tree)
				if err != nil {
					return restic.ID{}, errors.Wrapf(err, "cannot store new tree")
				}
				fmt.Printf("Saved tree %q\n", treeID)

				return treeID, nil
			case l > 1:
				return restic.ID{}, fmt.Errorf("%q should be a dir, but s a %q", item, node.Type)
			case node.Type != "file":
				return restic.ID{}, fmt.Errorf("%q should be a file, but is a %q", item, node.Type)
			}
		}
	}
	return restic.ID{}, fmt.Errorf("path %q not found in snapshot", item)
}

func runErase(opts EraseOptions, gopts GlobalOptions, args []string) error {
	ctx := gopts.ctx

	path := args[0]

	splittedPath := splitPath(path)

	repo, err := OpenRepository(gopts)
	if err != nil {
		return err
	}

	lock, err := lockRepoExclusive(repo)
	defer unlockRepo(lock)
	if err != nil {
		return err
	}

	err = repo.LoadIndex(ctx)
	if err != nil {
		return err
	}

	var id restic.ID

	snapshotIDString := opts.Snapshot
	if snapshotIDString == "latest" {
		id, err = restic.FindLatestSnapshot(ctx, repo, opts.Paths, opts.Tags, opts.Host)
		if err != nil {
			Exitf(1, "latest snapshot for criteria not found: %v Paths:%v Host:%v", err, opts.Paths, opts.Host)
		}
	} else {
		id, err = restic.FindSnapshot(repo, snapshotIDString)
		if err != nil {
			Exitf(1, "invalid id %q: %v", snapshotIDString, err)
		}
	}

	sn, err := restic.LoadSnapshot(gopts.ctx, repo, id)
	if err != nil {
		Exitf(2, "loading snapshot %q failed: %v", snapshotIDString, err)
	}

	tree, err := repo.LoadTree(ctx, *sn.Tree)
	if err != nil {
		Exitf(2, "loading tree for snapshot %q failed: %v", snapshotIDString, err)
	}

	erasedTree, err := eraseFromTree(ctx, tree, repo, "", splittedPath)
	if err != nil {
		Exitf(2, "cannot erase file: %v", err)
	}

	err = repo.Flush(ctx)
	if err != nil {
		Exitf(2, "cannot flush repo: %v", err)
	}

	err = repo.SaveIndex(ctx)
	if err != nil {
		Exitf(2, "cannot save index: %v", err)
	}

	erasedSnap, err := restic.NewSnapshot(sn.Paths, sn.Tags, sn.Hostname, sn.Time)
	erasedSnap.Excludes = sn.Excludes
	erasedSnap.Parent = sn.Parent
	erasedSnap.Tree = &erasedTree

	erasedID, err := repo.SaveJSONUnpacked(ctx, restic.SnapshotFile, erasedSnap)
	if err != nil {
		Exitf(2, "cannot save snapshot: %v", err)
	}
	fmt.Printf("New erased snapshot %q\n", erasedID)

	return nil
}
