package deploy

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/techblog/staticgen/pkg/config"
	"github.com/techblog/staticgen/pkg/logger"
)

type Deployer struct {
	cfg *config.Config
	log *logger.Logger
}

func New(cfg *config.Config, log *logger.Logger) *Deployer {
	return &Deployer{
		cfg: cfg,
		log: log,
	}
}

func (d *Deployer) Deploy() error {
	switch strings.ToLower(d.cfg.Deploy.Type) {
	case "dir":
		return d.deployToDir()
	case "git":
		return d.deployToGit()
	case "rsync":
		return d.deployRsync()
	case "s3":
		return d.deployS3Mock()
	default:
		return fmt.Errorf("unknown deploy type: %s", d.cfg.Deploy.Type)
	}
}

func (d *Deployer) deployToDir() error {
	target := d.cfg.Deploy.Target
	if target == "" {
		return fmt.Errorf("deploy target not specified")
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(d.cfg.Paths.Root, target)
	}

	d.log.Info("Deploying to directory: %s", target)

	if _, err := os.Stat(target); err == nil {
		d.log.Info("Cleaning target directory...")
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("clean target: %w", err)
		}
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("create target: %w", err)
	}

	source := d.cfg.Paths.Public
	count := 0

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(target, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})

	if err != nil {
		return err
	}

	d.log.Success("Deployed %d files to %s", count, target)
	return nil
}

func (d *Deployer) deployToGit() error {
	target := d.cfg.Deploy.Target
	if target == "" {
		return fmt.Errorf("deploy target not specified")
	}

	d.log.Info("Deploying via Git to: %s", target)

	branch := "gh-pages"
	if b, ok := d.cfg.Deploy.Options["branch"]; ok && b != "" {
		branch = b
	}

	message := fmt.Sprintf("Deploy at %s", time.Now().Format("2006-01-02 15:04:05"))
	if m, ok := d.cfg.Deploy.Options["message"]; ok && m != "" {
		message = m
	}

	tmpDir, err := os.MkdirTemp("", "staticgen-deploy-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	d.log.Info("Cloning repository...")
	cmd := exec.Command("git", "clone", "--branch", branch, "--single-branch", target, tmpDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		d.log.Warn("Branch %s not found, creating new...", branch)
		cmd = exec.Command("git", "clone", target, tmpDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}

		cmd = exec.Command("git", "checkout", "--orphan", branch)
		cmd.Dir = tmpDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git checkout orphan: %w", err)
		}

		cmd = exec.Command("git", "rm", "-rf", ".")
		cmd.Dir = tmpDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	d.log.Info("Copying files...")
	err = filepath.Walk(d.cfg.Paths.Public, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(d.cfg.Paths.Public, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(tmpDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})

	if err != nil {
		return fmt.Errorf("copy files: %w", err)
	}

	d.log.Info("Committing changes...")
	commands := [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", message, "--allow-empty"},
		{"git", "push", "origin", branch, "--force"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
		}
	}

	d.log.Success("Deployed successfully via Git")
	return nil
}

func (d *Deployer) deployRsync() error {
	target := d.cfg.Deploy.Target
	if target == "" {
		return fmt.Errorf("deploy target not specified")
	}

	d.log.Info("Deploying via rsync to: %s", target)

	source := d.cfg.Paths.Public + string(filepath.Separator)
	if !strings.HasSuffix(source, "/") {
		source += "/"
	}

	args := []string{
		"-avz",
		"--delete",
		"--exclude", ".git",
		source,
		target,
	}

	if extra, ok := d.cfg.Deploy.Options["args"]; ok && extra != "" {
		extraArgs := strings.Fields(extra)
		args = append(args[:1], append(extraArgs, args[1:]...)...)
	}

	cmd := exec.Command("rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync: %w", err)
	}

	d.log.Success("Deployed successfully via rsync")
	return nil
}

func (d *Deployer) deployS3Mock() error {
	target := d.cfg.Deploy.Target
	if target == "" {
		target = "./s3-mock"
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(d.cfg.Paths.Root, target)
	}

	d.log.Info("Deploying to S3 mock: %s", target)
	d.log.Warn("S3 mock mode - simulating S3 upload to local directory")

	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("create s3 mock dir: %w", err)
	}

	bucket := "staticgen-site"
	if b, ok := d.cfg.Deploy.Options["bucket"]; ok && b != "" {
		bucket = b
	}

	bucketDir := filepath.Join(target, bucket)
	if _, err := os.Stat(bucketDir); err == nil {
		os.RemoveAll(bucketDir)
	}
	if err := os.MkdirAll(bucketDir, 0755); err != nil {
		return fmt.Errorf("create bucket dir: %w", err)
	}

	fileCount := 0
	totalSize := int64(0)

	err := filepath.Walk(d.cfg.Paths.Public, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(d.cfg.Paths.Public, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(bucketDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		if err := copyFile(path, destPath); err != nil {
			return err
		}

		fileCount++
		totalSize += info.Size()

		contentType := getContentType(relPath)
		metaPath := destPath + ".meta"
		metaContent := fmt.Sprintf("Content-Type: %s\nContent-Length: %d\nLast-Modified: %s\n",
			contentType, info.Size(), info.ModTime().Format(time.RFC1123))
		os.WriteFile(metaPath, []byte(metaContent), 0644)

		d.log.Debug("  [S3] PUT s3://%s/%s (%s, %d bytes)", bucket, filepath.ToSlash(relPath), contentType, info.Size())

		return nil
	})

	if err != nil {
		return err
	}

	d.log.Success("S3 mock deploy complete: %d files, %d bytes uploaded to s3://%s/", fileCount, totalSize, bucket)
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	info, err := srcFile.Stat()
	if err != nil {
		return nil
	}
	return os.Chmod(dst, info.Mode())
}

func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	types := map[string]string{
		".html": "text/html; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".js":   "application/javascript; charset=utf-8",
		".json": "application/json; charset=utf-8",
		".xml":  "application/xml; charset=utf-8",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".txt":  "text/plain; charset=utf-8",
		".md":   "text/markdown; charset=utf-8",
	}
	if t, ok := types[ext]; ok {
		return t
	}
	return "application/octet-stream"
}
