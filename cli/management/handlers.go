package management

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	"github.com/asocpro/workshop-builder/cli/docker"
)

// ── Index ─────────────────────────────────────────────────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	currentID := s.currentID
	currentStep := s.currentStep
	image := s.workshopImage
	steps := s.steps
	workshopURL := s.workshopURL
	s.mu.Unlock()

	shortID := currentID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	// Build step list items
	var stepsHTML strings.Builder
	for _, step := range steps {
		isCurrent := step.ID == currentStep
		idEsc := html.EscapeString(step.ID)
		titleEsc := html.EscapeString(step.Title)

		currentClass := ""
		dotClass := "step-dot"
		actionHTML := `<span class="step-action"><button class="btn btn-sm">Go to step</button></span>`
		if isCurrent {
			currentClass = " current"
			dotClass = "step-dot active"
			actionHTML = `<span class="step-action"><span class="current-badge">Current</span></span>`
		}

		fmt.Fprintf(&stepsHTML, `
      <div class="step-item%s" data-step-id="%s" data-step-title="%s">
        <span class="%s"></span>
        <span class="step-info">
          <span class="step-id">%s</span>
          <span class="step-title">%s</span>
        </span>
        %s
      </div>`, currentClass, idEsc, titleEsc, dotClass, idEsc, titleEsc, actionHTML)
	}
	if len(steps) == 0 {
		stepsHTML.WriteString(`<div class="empty">No steps found in workshop.json</div>`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, page,
		html.EscapeString(image),       // 1: <title>
		html.EscapeString(workshopURL), // 2: Open Workshop href
		stepsHTML.String(),             // 3: step list
		html.EscapeString(image),       // 4: status bar image
		html.EscapeString(shortID),     // 5: status bar container
	)
}

// page is the full management UI — dark theme matching the workshop frontend.
// Placeholders (in order): image title, workshopURL href, workshopURL text,
// steps HTML, image status, container ID status.
const page = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Workshop Management — %s</title>
  <style>
    :root {
      --bg:        #111827;
      --surface:   #1f2937;
      --surface2:  #273347;
      --border:    #374151;
      --text:      #f9fafb;
      --muted:     #9ca3af;
      --accent:    #3b82f6;
      --accent-h:  #2563eb;
      --green:     #22c55e;
      --red:       #ef4444;
      --red-h:     #dc2626;
    }
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg); color: var(--text);
      font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
      font-size: 14px; line-height: 1.5;
      min-height: 100vh; padding: 28px 20px;
    }
    a { color: var(--accent); text-decoration: none; }
    a:hover { text-decoration: underline; }

    .wrap { max-width: 600px; margin: 0 auto; }

    /* ── Header ── */
    .header {
      display: flex; align-items: center;
      justify-content: space-between; margin-bottom: 24px;
    }
    .header h1 { font-size: 1.1rem; font-weight: 600; }
    .header-links { display: flex; gap: 16px; align-items: center; font-size: 13px; }
    .header-links a.workshop-link {
      display: inline-flex; align-items: center; gap: 5px;
      background: var(--accent); color: white; border-radius: 6px;
      padding: 5px 12px; font-weight: 500; font-size: 13px;
    }
    .header-links a.workshop-link:hover { background: var(--accent-h); text-decoration: none; }
    .header-links a.dim { color: var(--muted); }
    .header-links a.dim:hover { color: var(--text); }

    /* ── Cards ── */
    .card {
      background: var(--surface); border: 1px solid var(--border);
      border-radius: 8px; margin-bottom: 16px; overflow: hidden;
    }
    .card-header {
      padding: 10px 16px; border-bottom: 1px solid var(--border);
      font-size: 11px; font-weight: 600; letter-spacing: .06em;
      text-transform: uppercase; color: var(--muted);
    }

    /* ── Step list ── */
    .step-item {
      display: flex; align-items: center; gap: 12px;
      padding: 11px 16px; border-bottom: 1px solid var(--border);
    }
    .step-item:last-child { border-bottom: none; }
    .step-item.current { background: var(--surface2); }
    .step-dot {
      width: 9px; height: 9px; border-radius: 50%%;
      border: 2px solid var(--border); flex-shrink: 0;
    }
    .step-dot.active { background: var(--green); border-color: var(--green); }
    .step-info { flex: 1; display: flex; flex-direction: column; gap: 1px; }
    .step-id   { font-size: 11px; color: var(--muted); font-family: ui-monospace, monospace; }
    .step-title { font-weight: 500; }
    .current-badge {
      font-size: 11px; color: var(--green); font-weight: 600;
      letter-spacing: .04em; text-transform: uppercase;
    }
    .empty { padding: 16px; color: var(--muted); font-style: italic; }

    /* ── Buttons ── */
    .btn {
      cursor: pointer; border: 1px solid transparent; border-radius: 6px;
      padding: 6px 14px; font-size: 13px; font-weight: 500;
      background: var(--surface2); color: var(--text); border-color: var(--border);
      transition: background .12s, border-color .12s;
    }
    .btn:hover { background: var(--border); }
    .btn-sm { padding: 4px 10px; font-size: 12px; }
    .btn-danger { background: var(--red); color: white; border-color: var(--red); }
    .btn-danger:hover { background: var(--red-h); border-color: var(--red-h); }
    .btn-ghost { background: transparent; color: var(--muted); border-color: var(--border); }
    .btn-ghost:hover { color: var(--text); background: var(--surface2); }

    /* ── Settings ── */
    .settings-row {
      display: flex; align-items: center; gap: 14px; padding: 14px 16px;
    }
    .settings-label { flex: 1; }
    .settings-label span { display: block; font-size: 12px; color: var(--muted); margin-top: 2px; }
    /* Toggle — traditional pill shape with visible track */
    .toggle { position: relative; width: 44px; height: 24px; flex-shrink: 0; }
    .toggle input { opacity: 0; width: 0; height: 0; position: absolute; }
    .track {
      position: absolute; inset: 0; border-radius: 24px;
      background: #4b5563; border: 1px solid #6b7280;
      cursor: pointer; transition: background .2s, border-color .2s;
    }
    .toggle input:checked ~ .track { background: var(--green); border-color: var(--green); }
    .track::after {
      content: ""; position: absolute; left: 3px; top: 3px;
      width: 16px; height: 16px; border-radius: 50%%; background: white;
      box-shadow: 0 1px 3px rgba(0,0,0,.4);
      transition: transform .2s;
    }
    .toggle input:checked ~ .track::after { transform: translateX(20px); }

    /* ── Status bar ── */
    .status {
      font-size: 11px; color: var(--muted); padding: 0 4px;
      font-family: ui-monospace, monospace;
    }

    /* ── Modal ── */
    .overlay {
      position: fixed; inset: 0; background: rgba(0,0,0,.65);
      display: flex; align-items: center; justify-content: center;
      z-index: 50; padding: 20px;
    }
    .overlay.hidden { display: none; }
    .modal {
      background: var(--surface); border: 1px solid var(--border);
      border-radius: 10px; padding: 24px; max-width: 420px; width: 100%%;
    }
    .modal h2 { font-size: 1rem; font-weight: 600; margin-bottom: 10px; }
    .modal p { color: var(--muted); font-size: 13px; line-height: 1.65; margin-bottom: 22px; }
    .modal-actions { display: flex; gap: 10px; justify-content: flex-end; }
  </style>
</head>
<body>
<div class="wrap">

  <div class="header">
    <h1>Workshop Management</h1>
    <div class="header-links">
      <a class="workshop-link" href="%s" target="workshop">Open Workshop ↗</a>
      <a class="dim" href="/status">Status</a>
    </div>
  </div>

  <div class="card">
    <div class="card-header">Steps</div>
    %s
  </div>

  <div class="card">
    <div class="card-header">Settings</div>
    <div class="settings-row">
      <div class="settings-label">
        Skip confirmation when switching steps
        <span>Switch immediately without the reset warning dialog</span>
      </div>
      <label class="toggle">
        <input type="checkbox" id="skip" onchange="savePref(this)">
        <span class="track"></span>
      </label>
    </div>
  </div>

  <div class="status">Image: %s &nbsp;·&nbsp; Container: %s</div>

</div>

<!-- Confirmation modal -->
<div class="overlay hidden" id="overlay">
  <div class="modal" role="dialog" aria-modal="true">
    <h2 id="modal-title">Switch steps?</h2>
    <p id="modal-body">
      This will reset the workspace to its default state.
      Any files you have created or modified will be lost.
    </p>
    <div class="modal-actions">
      <button class="btn btn-ghost" onclick="closeModal()">Cancel</button>
      <button class="btn btn-danger" id="confirm-btn" onclick="confirmSwitch()">Switch Step</button>
    </div>
  </div>
</div>

<script>
  // Name this tab so the workshop's target="workshop-management" link navigates back here.
  window.name = 'workshop-management'

  // Wire "Go to step" buttons via event delegation — avoids onclick HTML escaping issues.
  document.querySelector('.card').addEventListener('click', function(e) {
    const btn = e.target.closest('button.btn-sm')
    if (!btn) return
    const item = btn.closest('.step-item')
    switchStep(item.dataset.stepId, item.dataset.stepTitle)
  })

  // Restore setting
  const skipEl = document.getElementById('skip')
  skipEl.checked = localStorage.getItem('mgmt_skip_confirm') === '1'

  function savePref(el) {
    localStorage.setItem('mgmt_skip_confirm', el.checked ? '1' : '0')
  }

  let pending = null

  function switchStep(id, title) {
    if (localStorage.getItem('mgmt_skip_confirm') === '1') {
      doSwitch(id)
      return
    }
    pending = id
    document.getElementById('modal-title').textContent = 'Switch to "' + title + '"?'
    document.getElementById('overlay').classList.remove('hidden')
    document.getElementById('confirm-btn').focus()
  }

  function closeModal() {
    document.getElementById('overlay').classList.add('hidden')
    pending = null
  }

  function confirmSwitch() {
    if (pending) doSwitch(pending)
  }

  function doSwitch(id) {
    closeModal()
    fetch('/step/' + id, { method: 'POST' })
      .then(r => { if (!r.ok) throw new Error(r.statusText); return r.json() })
      .then(data => markCurrent(data.step))
      .catch(err => alert('Error switching step: ' + err))
  }

  // Update the step list in place — no page reload, so the workshop tab is never touched.
  function markCurrent(stepId) {
    document.querySelectorAll('.step-item').forEach(el => {
      const isNow = el.dataset.stepId === stepId
      el.classList.toggle('current', isNow)
      el.querySelector('.step-dot').className = 'step-dot' + (isNow ? ' active' : '')
      const action = el.querySelector('.step-action')
      if (isNow) {
        action.innerHTML = '<span class="current-badge">Current</span>'
      } else {
        const btn = document.createElement('button')
        btn.className = 'btn btn-sm'
        btn.textContent = 'Go to step'
        action.replaceChildren(btn)
      }
    })
  }

  // Close on overlay backdrop click
  document.getElementById('overlay').addEventListener('click', function(e) {
    if (e.target === this) closeModal()
  })

  // Close on Escape
  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') closeModal()
  })
</script>
</body>
</html>`

// ── Step transition ────────────────────────────────────────────────────────────

func (s *Server) handleGoToStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	stepID := strings.TrimPrefix(r.URL.Path, "/step/")
	if stepID == "" {
		http.Error(w, "step ID required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	oldID := s.currentID
	workshopPort := s.workshopPort
	mgmtURL := s.mgmtURL
	newImage := fmt.Sprintf("%s:%s", s.workshopImage, stepID)
	s.mu.Unlock()

	log.Printf("Transitioning to step %s (image: %s)", stepID, newImage)
	ctx := context.Background()

	if oldID != "" {
		log.Printf("Stopping container %s", oldID)
		if err := s.dc.StopContainer(ctx, oldID); err != nil {
			log.Printf("warning: stopping old container: %v", err)
		}
	}

	newID, err := s.dc.RunContainer(ctx, docker.RunOptions{
		Image:         newImage,
		Name:          docker.GenerateName("workshop-workspace"),
		WorkshopPort:  workshopPort,
		ManagementURL: mgmtURL,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("starting new container: %v", err), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.currentID = newID
	s.currentStep = stepID
	s.mu.Unlock()

	log.Printf("New container running: %s (step: %s)", newID, stepID)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "ok",
		"containerID": newID,
		"step":        stepID,
		"image":       newImage,
	})
}

// ── Status ────────────────────────────────────────────────────────────────────

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	status := map[string]any{
		"workshopImage":     s.workshopImage,
		"currentContainer":  s.currentID,
		"currentStep":       s.currentStep,
		"workshopPort":      s.workshopPort,
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, status)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
