# M1 — Example Workshop (hello-linux)

## Goal

Create a hand-crafted example workshop under `examples/hello-linux/` that serves as the concrete fixture all subsequent milestones build against. Forces realistic content from day one.

## Prerequisites

None. This is the first milestone.

## Working Directory

`/home/zach/workshop-builder`

## Acceptance Test

```bash
ls examples/hello-linux/steps/
# → step-1-intro  step-2-files  step-3-validate

ls examples/hello-linux/steps/step-2-files/
# → step.yaml  content.md  goss.yaml  files/

cat examples/hello-linux/steps/step-2-files/files/hello.sh
# → #!/bin/bash ...
```

Visual inspection that all files exist and content is realistic.

## Partial State Already Exists

The following was already created in a prior session:
- `examples/hello-linux/workshop.yaml` — **COMPLETE** (verified)
- Directory structure created:
  - `examples/hello-linux/steps/step-1-intro/`
  - `examples/hello-linux/steps/step-2-files/files/`
  - `examples/hello-linux/steps/step-3-validate/`

**Only the step files need to be written.**

## Files to Create

### `examples/hello-linux/workshop.yaml` (already done — do not recreate)

```yaml
version: v1

workshop:
  name: hello-linux
  image: localhost/hello-linux
  navigation: linear

base:
  image: workshop-base:ubuntu

steps:
  - step-1-intro
  - step-2-files
  - step-3-validate
```

---

### Step 1: Intro

#### `examples/hello-linux/steps/step-1-intro/step.yaml`

```yaml
title: "Welcome to Hello Linux"
```

No files, no commands. Pure intro step.

#### `examples/hello-linux/steps/step-1-intro/content.md`

```markdown
# Welcome to Hello Linux

This workshop introduces you to working with files and scripts in a Linux environment.

## What You'll Learn

- [ ] Navigate the Linux filesystem
- [ ] Create and execute shell scripts
- [ ] Validate your work with automated checks

## Your Environment

You have a full Ubuntu Linux environment with a terminal below. Try running:

```bash
echo "Hello, World!"
```

When you're ready to move on, use the step navigation on the left.

> **Tip:** The terminal below is fully interactive — any command you run persists for this session.
```

#### `examples/hello-linux/steps/step-1-intro/hints.md`

```markdown
- Look at the terminal pane below the tutorial content
- Try typing `ls /workspace` to see what files exist
- Use `pwd` to check your current directory
```

---

### Step 2: Files

#### `examples/hello-linux/steps/step-2-files/step.yaml`

```yaml
title: "Working with Files"
files:
  - source: hello.sh
    target: /workspace/hello.sh
    mode: "0755"
commands:
  - chmod +x /workspace/hello.sh
```

#### `examples/hello-linux/steps/step-2-files/content.md`

```markdown
# Working with Files

In this step, you'll explore the `/workspace/hello.sh` script that has been placed in your environment.

## Your Task

1. Look at the script:
   ```bash
   cat /workspace/hello.sh
   ```

2. Run it:
   ```bash
   /workspace/hello.sh
   ```

3. Check that the file has execute permissions:
   ```bash
   ls -la /workspace/hello.sh
   ```

When all checks pass, click **Validate** to confirm your environment is set up correctly.

## What the Validator Checks

- `/workspace/hello.sh` exists
- `/workspace/hello.sh` is executable (has execute permission)
```

#### `examples/hello-linux/steps/step-2-files/goss.yaml`

```yaml
file:
  /workspace/hello.sh:
    exists: true
    mode: "0755"
```

#### `examples/hello-linux/steps/step-2-files/files/hello.sh`

```bash
#!/bin/bash
echo "Hello from the workshop!"
echo "Current user: $(whoami)"
echo "Current directory: $(pwd)"
```

---

### Step 3: Validate

#### `examples/hello-linux/steps/step-3-validate/step.yaml`

```yaml
title: "Validation and Completion"
commands:
  - echo validated > /workspace/done.txt
```

#### `examples/hello-linux/steps/step-3-validate/content.md`

```markdown
# Validation and Completion

In this final step, you'll verify that the workshop environment is working end-to-end.

## Your Task

Run the following command to mark this step as complete:

```bash
echo validated > /workspace/done.txt
```

Then check it worked:

```bash
cat /workspace/done.txt
```

Click **Validate** when ready.

## What the Validator Checks

- `/workspace/done.txt` exists and contains the text `validated`
```

#### `examples/hello-linux/steps/step-3-validate/goss.yaml`

```yaml
file:
  /workspace/done.txt:
    exists: true
    contains:
      - "validated"
```

#### `examples/hello-linux/steps/step-3-validate/hints.md`

```markdown
- The command `echo text > file` writes text to a file
- Use `>` to overwrite, `>>` to append
- Try: `echo validated > /workspace/done.txt`
```

#### `examples/hello-linux/steps/step-3-validate/solve.md`

```markdown
Run this exact command:

```bash
echo validated > /workspace/done.txt
```

Then validate.
```

---

## Key Points

- step-1-intro has NO goss.yaml (no validation button for that step)
- step-1-intro has hints.md (hints button shown)
- step-2-files has goss.yaml (validate button shown), no hints/solve
- step-3-validate has goss.yaml + hints.md + solve.md
- The `files/hello.sh` in step-2-files gets copied to `/workspace/hello.sh` inside the container image — it already exists when the student arrives at that step (because step images represent completed state, and students start from base image)
- Navigation is `linear` — students must complete steps in order

## Reference

- `docs/definition/workshop.md` — full schema for workshop.yaml and step.yaml
- `docs/artifact/flat-file-artifact.md` — what gets compiled into the image
