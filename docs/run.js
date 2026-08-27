/* ============================================================================
   run.js — دکمهٔ «▶ اجرا» برای بلاک‌های کد کلنگ
   ----------------------------------------------------------------------------
   - همهٔ <pre class="kolang-code"> را پیمایش می‌کند و دکمهٔ اجرا + پنل خروجی
     به آن‌ها اضافه می‌کند.
   - با نخستین کلیک، مفسر WASM را (به‌صورت تنبل) از ./wasm/ بارگذاری می‌کند.
   - کد را با globalThis.runKolang اجرا می‌کند و خروجی/خطا را نمایش می‌دهد.

   قرارداد WASM (از kolang-wasm/index.js):
     runKolang(code) -> Promise<{ ok: boolean, output: string, error: string }>
   ========================================================================== */

(function () {
  'use strict';

  var WASM_DIR = './wasm';
  var loadPromise = null;

  /* ---- بارگذاری تنبلِ WASM ---- */
  function loadWasm() {
    if (loadPromise) return loadPromise;
    loadPromise = new Promise(function (resolve, reject) {
      // ۱) ابتدا wasm_exec.js را بارگذاری کن تا globalThis.Go ساخته شود.
      var s = document.createElement('script');
      s.src = WASM_DIR + '/wasm_exec.js';
      s.onload = function () {
        if (typeof globalThis.Go !== 'function') {
          reject(new Error('کلاس Go پس از بارگذاری wasm_exec.js در دسترس نیست'));
          return;
        }
        // ۲) باینری WASM را بخوان و نمونه‌سازی کن.
        fetch(WASM_DIR + '/kolang.wasm')
          .then(function (r) {
            if (!r.ok) throw new Error('HTTP ' + r.status);
            return r.arrayBuffer();
          })
          .then(function (bytes) {
            var go = new globalThis.Go();
            return WebAssembly.instantiate(bytes, go.importObject).then(function (result) {
              // ۳) go.run مفسر را راه می‌اندازد و runKolang را روی globalThis
              //    ثبت می‌کند. این Promise هرگز پایان نمی‌یابد؛ پس await نمی‌کنیم.
              go.run(result.instance);
              waitForRunKolang(resolve, reject);
            });
          })
          .catch(reject);
      };
      s.onerror = function () {
        reject(new Error('بارگذاری wasm_exec.js ناموفق بود'));
      };
      document.head.appendChild(s);
    });
    return loadPromise;
  }

  function waitForRunKolang(resolve, reject) {
    var tries = 0;
    (function check() {
      if (typeof globalThis.runKolang === 'function') resolve();
      else if (tries++ > 200) reject(new Error('runKolang پس از راه‌اندازی WASM ثبت نشد'));
      else setTimeout(check, 50);
    })();
  }

  /* ---- راه‌اندازی هر بلاک کد ---- */
  function setupBlock(pre) {
    var code = pre.querySelector('code');
    if (!code) return;
    if (pre.closest('.code-block-wrapper')) return; // جلوگیری از دوبار اجرا

    var wrapper = document.createElement('div');
    wrapper.className = 'code-block-wrapper';
    pre.parentNode.insertBefore(wrapper, pre);
    wrapper.appendChild(pre);

    var btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'run-btn';
    btn.setAttribute('aria-label', 'اجرای این کد');
    btn.textContent = '▶ اجرا';
    wrapper.appendChild(btn);

    var out = document.createElement('div');
    out.className = 'code-output';
    out.hidden = true;
    wrapper.appendChild(out);

    btn.addEventListener('click', function () { runCode(code, btn, out); });
  }

  function runCode(codeEl, btn, out) {
    // textContent حتی پس از هایلایتِ Prism، کدِ خام را برمی‌گرداند.
    var src = codeEl.textContent;
    btn.disabled = true;
    out.hidden = false;
    setStatus(out, 'در حال بارگذاری...');

    loadWasm().then(function () {
      setStatus(out, 'در حال اجرا...');
      var result;
      try {
        result = globalThis.runKolang(src);
      } catch (e) {
        renderError(out, e);
        btn.disabled = false;
        return;
      }
      Promise.resolve(result).then(function (res) {
        renderOutput(out, res);
        btn.disabled = false;
      }).catch(function (err) {
        renderError(out, err);
        btn.disabled = false;
      });
    }).catch(function (err) {
      renderError(out, err, 'خطا در بارگذاری WASM');
      btn.disabled = false;
    });
  }

  function setStatus(out, msg) {
    out.innerHTML = '<span class="run-status"></span>';
    out.firstChild.textContent = msg;
  }

  function renderOutput(out, res) {
    out.innerHTML = '';
    if (res && res.ok) {
      var text = (res.output != null ? String(res.output) : '');
      if (text.length === 0) {
        var empty = document.createElement('span');
        empty.className = 'run-status run-empty';
        empty.textContent = 'پایان — بدون خروجی';
        out.appendChild(empty);
      } else {
        var pre = document.createElement('pre');
        pre.className = 'code-output-text';
        pre.textContent = text;
        out.appendChild(pre);
      }
    } else {
      var err = (res && res.error) ? res.error : 'خطای ناشناخته';
      var span = document.createElement('span');
      span.className = 'run-error';
      span.textContent = err;
      out.appendChild(span);
    }
  }

  function renderError(out, err, prefix) {
    out.innerHTML = '';
    var span = document.createElement('span');
    span.className = 'run-error';
    span.textContent = (prefix ? prefix + ': ' : 'خطا: ') + (err && err.message ? err.message : String(err));
    out.appendChild(span);
  }

  function init() {
    var blocks = document.querySelectorAll('pre.kolang-code');
    for (var i = 0; i < blocks.length; i++) setupBlock(blocks[i]);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
