// playground.js — محیط آزمایش کلنگ
//
// Loads CodeMirror 6 (from esm.sh CDN) with the Kolang language package,
// instantiates the Kolang WASM interpreter, and wires up the Run/Clear
// buttons, example snippets, theme toggle, and Ctrl+Enter shortcut.

import { EditorView, keymap, lineNumbers, highlightActiveLine } from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { bracketMatching, indentOnInput } from '@codemirror/language'
import { closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete'
import { highlightSelectionMatches, searchKeymap } from '@codemirror/search'
import { kolang, kolangCompletion, kolangTheme } from './kolang-language.js'


// ---------------------------------------------------------------------------
// Example snippets
// ---------------------------------------------------------------------------

const EXAMPLES = {
  hello: `/ اولین برنامه در کلنگ
«سلام دنیا!» بنویس`,

  math: `/ حساب ساده
مجموع = ۱ + ۲
مجموع بنویس                          / خروجی: ۳

توان = ۲ * ۱۰
توان بنویس                           / خروجی: ۱۰۲۴

تقسیم = ۷ ÷ ۲
تقسیم بنویس                          / خروجی: ۳.۵`,

  loop: `/ حلقهٔ شمارشی
برای ای از ۱ تا ۵:
    ای بنویس`,

  func: `/ تعریف تابع
تعریف جمع(الف و ب):
    الف + ب برگردان

جمع(۳ و ۴) بنویس                     / خروجی: ۷`,

  oop: `/ شیءگرایی
گونه سگ:
    تعریف صدادهیِ(خود):
        «واف واف» بنویس

رکس = سگ()
صدادهیِ()رکس`,

  concurrency: `/ همزمانی با کانال
ch = کانال(صحیح و ۱)

تعریف تولید():
    ch << ۴۲
    ch ببند

برو تولید()
مقدار = >>ch
مقدار بنویس`,
}

const DEFAULT_CODE = EXAMPLES.hello


// ---------------------------------------------------------------------------
// DOM references
// ---------------------------------------------------------------------------

const editorEl = document.getElementById('editor')
const outputEl = document.getElementById('output')
const runBtn = document.getElementById('run-btn')
const clearBtn = document.getElementById('clear-btn')
const loadingOverlay = document.getElementById('loading-overlay')
const themeToggle = document.getElementById('theme-toggle')


// ---------------------------------------------------------------------------
// Theme toggle (shares localStorage key 'kolang-theme' with the docs site)
// ---------------------------------------------------------------------------

function applyTheme() {
  let saved = null
  try { saved = localStorage.getItem('kolang-theme') } catch (_) {}
  if (saved === 'light') {
    document.documentElement.setAttribute('data-theme', 'light')
    themeToggle.setAttribute('aria-pressed', 'true')
  } else {
    document.documentElement.removeAttribute('data-theme')
    themeToggle.setAttribute('aria-pressed', 'false')
  }
}

themeToggle.addEventListener('click', () => {
  const isLight = document.documentElement.getAttribute('data-theme') === 'light'
  if (isLight) {
    document.documentElement.removeAttribute('data-theme')
    try { localStorage.setItem('kolang-theme', 'dark') } catch (_) {}
  } else {
    document.documentElement.setAttribute('data-theme', 'light')
    try { localStorage.setItem('kolang-theme', 'light') } catch (_) {}
  }
  applyTheme()
})

applyTheme()


// ---------------------------------------------------------------------------
// Output helpers
// ---------------------------------------------------------------------------

function setOutputPlaceholder() {
  outputEl.classList.remove('pg-running')
  outputEl.innerHTML = ''
  const span = document.createElement('span')
  span.className = 'pg-output-placeholder'
  span.textContent = 'برای اجرا، دکمهٔ «اجرا» را بزنید (Ctrl+Enter)'
  outputEl.appendChild(span)
}

function setRunning() {
  outputEl.classList.add('pg-running')
  outputEl.innerHTML = ''
  outputEl.textContent = 'در حال اجرا…'
}

function showResult(output, error) {
  outputEl.classList.remove('pg-running')
  outputEl.innerHTML = ''

  if (output) {
    const out = document.createElement('span')
    out.className = 'pg-stdout'
    out.textContent = output
    outputEl.appendChild(out)
  }

  if (error) {
    const err = document.createElement('span')
    err.className = 'pg-error-line'
    err.textContent = error
    outputEl.appendChild(err)
  }

  if (!output && !error) {
    const empty = document.createElement('span')
    empty.className = 'pg-output-placeholder'
    empty.textContent = '(خروجی خالی)'
    outputEl.appendChild(empty)
  }
}

function showError(message) {
  outputEl.classList.remove('pg-running')
  outputEl.innerHTML = ''
  const err = document.createElement('span')
  err.className = 'pg-error-line'
  err.textContent = message
  outputEl.appendChild(err)
}


// ---------------------------------------------------------------------------
// CodeMirror 6 editor setup
// ---------------------------------------------------------------------------

let editor = null

function createEditor() {
  editor = new EditorView({
    parent: editorEl,
    state: EditorState.create({
      doc: DEFAULT_CODE,
      extensions: [
        lineNumbers(),
        history(),
        highlightActiveLine(),
        bracketMatching(),
        indentOnInput(),
        closeBrackets(),
        highlightSelectionMatches(),
        kolangCompletion(),
        kolang(),
        ...kolangTheme(),
        EditorView.lineWrapping,
        keymap.of([
          indentWithTab,
          ...defaultKeymap,
          ...historyKeymap,
          ...closeBracketsKeymap,
          ...searchKeymap,
          { key: 'Mod-Enter', run: () => { runCode(); return true } },
        ]),
      ],
    }),
  })
}

function getEditorCode() {
  return editor.state.doc.toString()
}

function setEditorCode(text) {
  editor.dispatch({
    changes: { from: 0, to: editor.state.doc.length, insert: text },
  })
}


// ---------------------------------------------------------------------------
// WASM loading
// ---------------------------------------------------------------------------

let wasmReady = false

function hideLoading() {
  loadingOverlay.classList.add('pg-hidden')
}

function showLoadError(message) {
  const card = loadingOverlay.querySelector('.pg-loading-card')
  card.classList.add('pg-error')
  card.querySelector('.pg-loading-text').textContent = 'خطا در بارگذاری مفسر'
  card.querySelector('.pg-loading-sub').textContent = message
}

async function loadWasm() {
  const GoCtor = globalThis.Go
  if (!GoCtor) {
    showLoadError('wasm_exec.js بارگذاری نشد')
    return
  }

  const go = new GoCtor()
  const wasmUrl = 'wasm/kolang.wasm'

  // Try streaming first (needs application/wasm MIME type), fall back to
  // fetch + arrayBuffer for servers that don't set the right content type.
  let instance
  try {
    const result = await WebAssembly.instantiateStreaming(fetch(wasmUrl), go.importObject)
    instance = result.instance
  } catch (_) {
    try {
      const resp = await fetch(wasmUrl)
      const bytes = await resp.arrayBuffer()
      const result = await WebAssembly.instantiate(bytes, go.importObject)
      instance = result.instance
    } catch (err) {
      showLoadError('فایل kolang.wasm یافت نشد یا نامعتبر است')
      return
    }
  }

  // go.run starts the Go runtime; it returns a Promise that resolves when
  // the Go program exits — but our program blocks on select{} forever,
  // keeping the runtime alive. We don't need to await it.
  go.run(instance)

  // The Go main() registers globalThis.runKolang during startup. Poll
  // until it's available, then enable the Run button.
  const checkReady = () => {
    if (typeof globalThis.runKolang === 'function') {
      wasmReady = true
      runBtn.disabled = false
      hideLoading()
    } else {
      setTimeout(checkReady, 50)
    }
  }
  checkReady()
}


// ---------------------------------------------------------------------------
// Run code
// ---------------------------------------------------------------------------

async function runCode() {
  if (!wasmReady) return
  if (typeof globalThis.runKolang !== 'function') return

  const code = getEditorCode()
  setRunning()

  // Let the browser paint the "running" state before the (potentially
  // CPU-bound) WASM evaluation begins.
  await new Promise((r) => requestAnimationFrame(r))

  try {
    const result = await globalThis.runKolang(code)
    if (result && typeof result === 'object') {
      showResult(result.output || '', result.error || '')
    } else {
      showError('پاسخ غیرمنتظره از مفسر')
    }
  } catch (e) {
    showError(String(e))
  }
}


// ---------------------------------------------------------------------------
// Wire up UI events
// ---------------------------------------------------------------------------

runBtn.addEventListener('click', runCode)

clearBtn.addEventListener('click', () => {
  setOutputPlaceholder()
})

// Example buttons
document.querySelectorAll('.pg-example-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    const key = btn.dataset.example
    const code = EXAMPLES[key]
    if (code) {
      setEditorCode(code)
      editorEl.focus()
    }
  })
})


// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

createEditor()
loadWasm()
