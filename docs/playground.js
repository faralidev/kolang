// playground.js — محیط آزمایش کلنگ
//
// Textarea + syntax-highlighting overlay editor (no CodeMirror dependency).
// Loads the Kolang WASM interpreter, handles Run/Clear, examples, theme,
// and the Ctrl+Enter keyboard shortcut.

(function () {
  'use strict'


  // =========================================================================
  // Syntax highlighter — produces HTML with kolang-* token spans.
  // The colors come from kolang-syntax-theme.css (linked in playground.html).
  // =========================================================================

  var KEYWORDS = {
    'اگر': 1, 'وگرنه': 1, 'تاوقتی': 1, 'برای': 1, 'از': 1, 'تا': 1, 'گام': 1,
    'در': 1, 'بپا': 1, 'درنهایت': 1, 'اتمام': 1, 'بروبعدی': 1,
    'تعریف': 1, 'گونه': 1, 'رابط': 1, 'وارث': 1, 'رهی': 1,
    'بانام': 1, 'به': 1, 'و': 1, 'پوشش': 1, 'با': 1,
    'همچنین': 1, 'یا': 1, 'باشد': 1, 'نباشد': 1,
    'جهانی': 1, 'نامحلی': 1, 'مثل': 1, 'ساخت': 1,
  }

  var BUILTINS = {
    'بنویس': 1, 'برگردان': 1, 'بیافزا': 1, 'حذف‌کن': 1, 'حذفکن': 1,
    'بده': 1, 'بیار': 1, 'بگیر': 1, 'ببند': 1, 'بساز': 1, 'بساز‌از': 1,
    'برو': 1, 'تأخیری': 1, 'بسته‌است': 1, 'طول': 1, 'نوع': 1, 'بازه': 1,
    'جمع': 1, 'کمینه': 1, 'بیشینه': 1, 'مرتب': 1, 'مطلق': 1, 'گرد': 1,
    'معکوس': 1, 'شمارش': 1, 'بقچه': 1, 'نگاشت': 1, 'پالایش': 1,
    'ویژگی': 1, 'دارد': 1, 'تنظیم‌ویژگی': 1, 'هویت': 1, 'اجرا': 1,
    'خطا': 1, 'کانال': 1, 'فهرست': 1, 'گنجه': 1, 'قفسه': 1, 'مجموعه': 1,
    'صحیح': 1, 'اعشاری': 1, 'متن': 1, 'بولی': 1, 'بازکردن': 1, 'هر': 1,
  }

  var BOOLEANS = { 'درست': 1, 'غلط': 1 }
  var NULLS = { 'تهی': 1 }
  var SELFS = { 'خود': 1, 'والد': 1 }
  var EXCEPTIONS = {
    'خطای‌صفر': 1, 'خطای‌مقدار': 1, 'خطای‌نوع': 1, 'خطای‌کلید': 1,
    'خطای‌نمایه': 1, 'خطای‌فایل': 1, 'استثنا': 1, 'خطا': 1,
  }
  var CLASS_INTRO = { 'گونه': 1, 'رابط': 1, 'وارث': 1 }
  var MODULES = {
    'ریاضی': 1, 'تصادفی': 1, 'زمان': 1, 'تقویم': 1, 'سیستم': 1, 'مسیر': 1,
    'سیستم‌عامل': 1, 'رشته‌ها': 1, 'عبارت‌منظم': 1, 'رجکس': 1, 'جیسون': 1,
    'اینترنت': 1, 'درخواست': 1,
  }

  var OPERATORS = [
    '÷/=', '**=', '÷=', '**', '÷/', '<<', '>>', '->', '|>',
    '==', '<=', '>=', '+=', '-=', '*=', '%=',
    '÷', '×', '<', '>', '=', '%', '+', '-', '*',
  ]

  var IDENT_START = /[\u0621-\u064A\u0670-\u06FFA-Za-z_]/
  var IDENT_CHAR = /[\u0621-\u064A\u0670-\u06FFA-Za-z0-9_\u200C]/
  var DIGIT = /[\u06F0-\u06F90-9]/

  function escapeHtml(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  }

  function highlightKolang(code) {
    var html = ''
    var i = 0
    var n = code.length
    var expectType = false

    while (i < n) {
      var ch = code[i]

      // Whitespace — pass through
      if (ch === ' ' || ch === '\n' || ch === '\t' || ch === '\r') {
        html += ch
        i++
        continue
      }

      // Block comment: // ... //
      if (ch === '/' && code[i + 1] === '/') {
        var closeIdx = code.indexOf('//', i + 2)
        var nlIdx = code.indexOf('\n', i + 2)
        var end
        if (closeIdx !== -1 && (nlIdx === -1 || closeIdx < nlIdx)) {
          end = closeIdx + 2
        } else if (nlIdx !== -1) {
          end = nlIdx
        } else {
          end = n
        }
        html += '<span class="kolang-comment">' + escapeHtml(code.slice(i, end)) + '</span>'
        i = end
        continue
      }

      // Line comment: / to end of line
      if (ch === '/') {
        var le = code.indexOf('\n', i)
        if (le === -1) le = n
        html += '<span class="kolang-comment">' + escapeHtml(code.slice(i, le)) + '</span>'
        i = le
        continue
      }

      // String: « ... »
      if (ch === '«') {
        var se = code.indexOf('»', i + 1)
        if (se === -1) se = n
        else se++
        html += '<span class="kolang-string">' + escapeHtml(code.slice(i, se)) + '</span>'
        i = se
        continue
      }

      // Number — hex / binary / octal prefix or plain decimal
      if (DIGIT.test(ch)) {
        if ((ch === '۰' || ch === '0') && /[xXbBoO]/.test(code[i + 1] || '')) {
          var j = i + 2
          while (j < n && /[0-9a-fA-F\u06F0-\u06F901۰-۷]/.test(code[j])) j++
          html += '<span class="kolang-number">' + escapeHtml(code.slice(i, j)) + '</span>'
          i = j
          continue
        }
        var j2 = i
        while (j2 < n) {
          var c = code[j2]
          if (DIGIT.test(c) || c === '٬' || c === ',' || c === '.' || c === '٫') {
            j2++
          } else if ((c === '+' || c === '-') && /[eE]/.test(code[j2 - 1] || '')) {
            j2++
          } else if (/[eE]/.test(c) && j2 > i) {
            j2++
          } else {
            break
          }
        }
        html += '<span class="kolang-number">' + escapeHtml(code.slice(i, j2)) + '</span>'
        i = j2
        continue
      }

      // Ezafe — U+0650 kasra
      if (ch === '\u0650') {
        html += '<span class="kolang-ezafe">\u0650</span>'
        i++
        continue
      }

      // Operators — longest match first
      var opMatched = false
      for (var oi = 0; oi < OPERATORS.length; oi++) {
        var op = OPERATORS[oi]
        if (code.substr(i, op.length) === op) {
          html += '<span class="kolang-operator">' + escapeHtml(op) + '</span>'
          i += op.length
          opMatched = true
          break
        }
      }
      if (opMatched) continue

      // Identifier
      if (IDENT_START.test(ch)) {
        var k = i
        while (k < n && IDENT_CHAR.test(code[k])) k++
        var word = code.slice(i, k)
        var cls = null

        if (expectType) {
          cls = 'kolang-class-name'
          expectType = false
        } else if (CLASS_INTRO[word]) {
          cls = 'kolang-keyword'
          expectType = true
        } else if (KEYWORDS[word]) {
          cls = 'kolang-keyword'
        } else if (BUILTINS[word]) {
          cls = 'kolang-builtin'
        } else if (BOOLEANS[word]) {
          cls = 'kolang-boolean'
        } else if (NULLS[word]) {
          cls = 'kolang-null'
        } else if (SELFS[word]) {
          cls = 'kolang-self'
        } else if (EXCEPTIONS[word]) {
          cls = 'kolang-error-type'
        } else if (MODULES[word]) {
          cls = 'kolang-module-name'
        }

        if (cls) {
          html += '<span class="' + cls + '">' + escapeHtml(word) + '</span>'
        } else {
          html += escapeHtml(word)
        }
        i = k
        continue
      }

      // Punctuation and any other character — pass through (escaped)
      html += escapeHtml(ch)
      i++
    }

    return html
  }


  // =========================================================================
  // Example snippets
  // =========================================================================

  var EXAMPLES = {
    hello: '/ اولین برنامه در کلنگ\n«سلام دنیا!» بنویس',

    math: '/ حساب ساده\nمجموع = ۱ + ۲\nمجموع بنویس                          / خروجی: ۳\n\nتوان = ۲ * ۱۰\nتوان بنویس                           / خروجی: ۱۰۲۴\n\nتقسیم = ۷ ÷ ۲\nتقسیم بنویس                          / خروجی: ۳.۵',

    loop: '/ حلقهٔ شمارشی\nبرای ای از ۱ تا ۵:\n    ای بنویس',

    func: '/ تعریف تابع\nتعریف جمع(الف و ب):\n    الف + ب برگردان\n\nجمع(۳ و ۴) بنویس                     / خروجی: ۷',

    oop: '/ شیءگرایی\nگونه سگ:\n    تعریف صدادهیِ(خود):\n        «واف واف» بنویس\n\nرکس = سگ()\nصدادهیِ()رکس',

    concurrency: '/ همزمانی با کانال\nch = کانال(صحیح و ۱)\n\nتعریف تولید():\n    ch << ۴۲\n    ch ببند\n\nبرو تولید()\nمقدار = >>ch\nمقدار بنویس',
  }

  var DEFAULT_CODE = EXAMPLES.hello


  // =========================================================================
  // DOM references
  // =========================================================================

  var editorInput = document.getElementById('editor-input')
  var editorHighlight = document.getElementById('editor-highlight-code')
  var editorPre = document.querySelector('.pg-editor-highlight')
  var outputEl = document.getElementById('output')
  var runBtn = document.getElementById('run-btn')
  var clearBtn = document.getElementById('clear-btn')
  var loadingOverlay = document.getElementById('loading-overlay')
  var themeToggle = document.getElementById('theme-toggle')


  // =========================================================================
  // Theme toggle — shares localStorage key 'kolang-theme' with docs site
  // =========================================================================

  function applyTheme() {
    var saved = null
    try { saved = localStorage.getItem('kolang-theme') } catch (e) {}
    if (saved === 'light') {
      document.documentElement.setAttribute('data-theme', 'light')
      themeToggle.setAttribute('aria-pressed', 'true')
    } else {
      document.documentElement.removeAttribute('data-theme')
      themeToggle.setAttribute('aria-pressed', 'false')
    }
  }

  themeToggle.addEventListener('click', function () {
    var isLight = document.documentElement.getAttribute('data-theme') === 'light'
    if (isLight) {
      document.documentElement.removeAttribute('data-theme')
      try { localStorage.setItem('kolang-theme', 'dark') } catch (e) {}
    } else {
      document.documentElement.setAttribute('data-theme', 'light')
      try { localStorage.setItem('kolang-theme', 'light') } catch (e) {}
    }
    applyTheme()
  })

  applyTheme()


  // =========================================================================
  // Editor — textarea + highlighting overlay
  // =========================================================================

  function syncHighlight() {
    var code = editorInput.value
    // Trailing newline keeps the last line visible in the overlay
    editorHighlight.innerHTML = highlightKolang(code) + '\n'
  }

  function syncScroll() {
    editorPre.scrollTop = editorInput.scrollTop
    editorPre.scrollLeft = editorInput.scrollLeft
  }

  editorInput.addEventListener('input', syncHighlight)
  editorInput.addEventListener('scroll', syncScroll)

  // Tab key — insert 4 spaces
  editorInput.addEventListener('keydown', function (e) {
    if (e.key === 'Tab') {
      e.preventDefault()
      var start = editorInput.selectionStart
      var end = editorInput.selectionEnd
      var val = editorInput.value
      editorInput.value = val.substring(0, start) + '    ' + val.substring(end)
      editorInput.selectionStart = editorInput.selectionEnd = start + 4
      syncHighlight()
    }
    // Ctrl+Enter / Cmd+Enter to run
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault()
      runCode()
    }
  })

  // Initialize editor content
  editorInput.value = DEFAULT_CODE
  syncHighlight()


  // =========================================================================
  // Output helpers
  // =========================================================================

  function setOutputPlaceholder() {
    outputEl.classList.remove('pg-running')
    outputEl.innerHTML = ''
    var span = document.createElement('span')
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
      var out = document.createElement('span')
      out.className = 'pg-stdout'
      out.textContent = output
      outputEl.appendChild(out)
    }

    if (error) {
      var err = document.createElement('span')
      err.className = 'pg-error-line'
      err.textContent = error
      outputEl.appendChild(err)
    }

    if (!output && !error) {
      var empty = document.createElement('span')
      empty.className = 'pg-output-placeholder'
      empty.textContent = '(خروجی خالی)'
      outputEl.appendChild(empty)
    }
  }

  function showError(message) {
    outputEl.classList.remove('pg-running')
    outputEl.innerHTML = ''
    var err = document.createElement('span')
    err.className = 'pg-error-line'
    err.textContent = message
    outputEl.appendChild(err)
  }


  // =========================================================================
  // WASM loading
  // =========================================================================

  var wasmReady = false

  function hideLoading() {
    loadingOverlay.classList.add('pg-hidden')
  }

  function showLoadError(message) {
    var card = loadingOverlay.querySelector('.pg-loading-card')
    card.classList.add('pg-error')
    card.querySelector('.pg-loading-text').textContent = 'خطا در بارگذاری مفسر'
    card.querySelector('.pg-loading-sub').textContent = message
  }

  function loadWasm() {
    if (typeof globalThis.Go === 'undefined') {
      showLoadError('wasm_exec.js بارگذاری نشد')
      return
    }

    var go = new globalThis.Go()
    var wasmUrl = 'wasm/kolang.wasm'

    // Try streaming first (needs application/wasm MIME), fall back to
    // fetch + arrayBuffer for servers that don't set the right content type.
    var instantiatePromise
    if (typeof WebAssembly.instantiateStreaming === 'function') {
      instantiatePromise = WebAssembly.instantiateStreaming(
        fetch(wasmUrl), go.importObject
      ).catch(function () {
        return fetch(wasmUrl).then(function (r) { return r.arrayBuffer() })
          .then(function (bytes) { return WebAssembly.instantiate(bytes, go.importObject) })
      })
    } else {
      instantiatePromise = fetch(wasmUrl).then(function (r) { return r.arrayBuffer() })
        .then(function (bytes) { return WebAssembly.instantiate(bytes, go.importObject) })
    }

    instantiatePromise.then(function (result) {
      var instance = result.instance || result
      // go.run starts the Go runtime. It returns a Promise that resolves when
      // the Go program exits — but our program blocks on select{} forever,
      // keeping the runtime alive. Don't await it.
      go.run(instance)

      // Poll until runKolang is registered by Go's main()
      var checkReady = setInterval(function () {
        if (typeof globalThis.runKolang === 'function') {
          clearInterval(checkReady)
          wasmReady = true
          runBtn.disabled = false
          hideLoading()
        }
      }, 50)
    }).catch(function (err) {
      showLoadError('فایل kolang.wasm یافت نشد یا نامعتبر است')
      console.error('WASM instantiation failed:', err)
    })
  }


  // =========================================================================
  // Run code
  // =========================================================================

  function runCode() {
    if (!wasmReady || typeof globalThis.runKolang !== 'function') return

    var code = editorInput.value
    setRunning()

    // Let the browser paint the "running" state before the (potentially
    // CPU-bound) WASM evaluation begins.
    requestAnimationFrame(function () {
      globalThis.runKolang(code).then(function (result) {
        if (result && typeof result === 'object') {
          showResult(result.output || '', result.error || '')
        } else {
          showError('پاسخ غیرمنتظره از مفسر')
        }
      }).catch(function (e) {
        showError(String(e))
      })
    })
  }


  // =========================================================================
  // Wire up UI events
  // =========================================================================

  runBtn.addEventListener('click', runCode)

  clearBtn.addEventListener('click', function () {
    setOutputPlaceholder()
  })

  // Example buttons
  document.querySelectorAll('.pg-example-btn').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var key = btn.dataset.example
      var code = EXAMPLES[key]
      if (code) {
        editorInput.value = code
        syncHighlight()
        editorInput.focus()
      }
    })
  })


  // =========================================================================
  // Init
  // =========================================================================

  loadWasm()
})()
