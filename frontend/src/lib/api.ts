export interface WorkshopState {
  activeStep: string
  completedSteps: string[]
  navigationMode: 'linear' | 'free' | 'guided'
  managementURL?: string
}

export interface StepListItem {
  id: string
  title: string
  group?: string
  requires?: string[]
  position: number
  accessible: boolean
  completed: boolean
  hasGoss: boolean
  hasHints: boolean
  hasExplain: boolean
  hasSolve: boolean
}

export interface ValidateResult {
  passed: boolean
  checks: CheckResult[]
}

export interface CheckResult {
  name: string
  passed: boolean
  summary?: string
}

export interface Command {
  ts: string
  cmd: string
  exit: number
}

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, options)
  if (!res.ok) {
    throw new Error(`${options?.method ?? 'GET'} ${path}: ${res.status} ${res.statusText}`)
  }
  return res.json()
}

export const api = {
  getState(): Promise<WorkshopState> {
    return fetchJSON('/api/state')
  },

  listSteps(): Promise<StepListItem[]> {
    return fetchJSON('/api/steps')
  },

  getStepContent(id: string): Promise<string> {
    return fetch(`/api/steps/${id}/content`).then(r => {
      if (!r.ok) throw new Error(`content ${id}: ${r.status}`)
      return r.text()
    })
  },

  navigate(id: string): Promise<{ activeStep: string }> {
    return fetchJSON(`/api/steps/${id}/navigate`, { method: 'POST' })
  },

  validate(id: string): Promise<ValidateResult> {
    return fetchJSON(`/api/steps/${id}/validate`, { method: 'POST' })
  },

  getCommands(limit = 50): Promise<Command[]> {
    return fetchJSON(`/api/commands?limit=${limit}`)
  },
}
