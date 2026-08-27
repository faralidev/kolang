/* ============================================================================
   highlight.js — بارگذاری Prism و تعریف زبان کلنگ
   ----------------------------------------------------------------------------
   - Prism core را از CDN بارگذاری می‌کند.
   - Prism.languages.kolang را با کلمات کلیدیِ keywords.json تعریف می‌کند.
   - همهٔ بلاک‌های <pre class="kolang-code"><code> را هایلایت می‌کند.

   اگر Prism از CDN بارگذاری نشد، بلاک‌ها همچنان با spanهای دستیِ
   class="kolang-*" (و رنگ‌های متناظر در docs.css) نمایش داده می‌شوند؛
   یعنی هایلایت بدون Prism هم کار می‌کند.

   منبع کلمات کلیدی: kolang/keywords.json (۵۲ مدخل) — دستی ویرایش نکنید.
   ========================================================================== */

(function () {
  'use strict';

  /* ---- کلمات کلیدی (از keywords.json، ۵۲ مدخل) ----
     درست/غلط به‌عنوان boolean و تهی به‌عنوان nil جدا شده‌اند تا رنگ متمایز
     بگیرند، اما همه از همان فهرست keywords.json هستند. */
  var KEYWORDS = [
    'بسته\u200cاست', 'بساز\u200cاز', 'تأخیری', 'بروبعدی', 'تاوقتی',
    'درنهایت', 'بانام', 'بیافزا', 'حذف\u200cکن', 'حذفکن', 'نامحلی',
    'نباشد', 'وارث', 'وگرنه', 'پوشش', 'کانال', 'گونه', 'برگردان',
    'اتمام', 'باشد', 'ببند', 'بده', 'برای', 'تعریف', 'جهانی', 'رهی',
    'مثل', 'همچنین', 'والد', 'گام', 'است', 'اگر', 'از', 'با', 'برو',
    'بساز', 'بنویس', 'به', 'بپا', 'بگیر', 'بیار', 'تا', 'خود', 'در',
    'رابط', 'و', 'یا'
  ];

  var BOOLEAN = ['درست', 'غلط'];
  var NIL = ['تهی'];

  /* ---- توابع آماده و نام انواع (غیر از کلمات کلیدی) ----
     از جدول مرجع در مستندات. */
  var BUILTINS = [
    'تنظیم\u200cویژگی', 'بیشینه', 'پالایش', 'نگاشت', 'شمارش', 'مجموعه',
    'اعشاری', 'بقچه', 'بولی', 'خطا', 'گرد', 'هر', 'فهرست', 'قفسه',
    'کمینه', 'متن', 'مطلق', 'معکوس', 'مرتب', 'نوع', 'هویت', 'اجرا',
    'بازه', 'دارد', 'صحیح', 'طول', 'گنجه', 'ویژگی', 'جمع'
  ];

  /* بازهٔ نویسه‌های «شناسه»: حروف فارسی/عربی (بدون اعراب و کسرهٔ اضافه)，
     نیم‌فاصله، لاتین، ارقام، زیرخط. کسرهٔ اضافه (U+0650) عمداً بیرون است
     تا به‌عنوان عملگرِ جداگانه هایلایت شود. */
  var ID = '\u0621-\u064F\u0670-\u06FF\u200CA-Za-z0-9_';

  function alt(words) {
    // حذف تکراری + مرتب بر اساس طول (نزولی) تا «برو» درون «بروبعدی» تطابق نخورد.
    var uniq = [], seen = {};
    words.forEach(function (w) { if (w && !seen[w]) { seen[w] = 1; uniq.push(w); } });
    uniq.sort(function (a, b) { return b.length - a.length; });
    return uniq.map(function (w) {
      return w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }).join('|');
  }

  /* الگوی مرز با قرارداد lookbehind ساختگیِ Prism:
       (^|[^ID])  کلمه  (?![ID])
     یعنی کلمه باید پس از آغازِ متن یا یک نویسهٔ غیرشناسه بیاید و پس از آن
     نباید نویسهٔ شناسه‌ای بیاید (تا «برو» درون «بروبعدی» تطابق نخورد). */
  function boundaryPattern(words) {
    return new RegExp('(^|[^' + ID + '])(?:' + alt(words) + ')(?![' + ID + '])', '');
  }

  function defineKolang() {
    if (!window.Prism || Prism.languages.kolang) return;

    Prism.languages.kolang = {
      'comment': /[#\/][^\n]*/,
      'string': {
        pattern: /«[^»]*»/,
        greedy: true
      },
      'number': /(?:[0\u06F0][xX][0-9A-Fa-f]+|[0\u06F0][bB][01\u06F0\u06F1]+|[0\u06F0][oO][0-7\u06F0-\u06F7]+|[\u06F0-\u06F90-9][\u06F0-\u06F90-9\u066B\u066C.,]*)/,
      'boolean': { pattern: boundaryPattern(BOOLEAN), lookbehind: true },
      'nil':     { pattern: boundaryPattern(NIL),     lookbehind: true },
      'keyword': { pattern: boundaryPattern(KEYWORDS), lookbehind: true },
      'builtin': { pattern: boundaryPattern(BUILTINS), lookbehind: true },
      'operator': /\u00F7\/|<<|>>|\|>|->|[-+\u00D7\u00F7*%<>=!:.]+/,
      'ezafe': /\u0650/,
      'punctuation': /[(){}\[\]]/
    };
  }

  function highlightAll() {
    var blocks = document.querySelectorAll('pre.kolang-code code');
    for (var i = 0; i < blocks.length; i++) {
      try {
        var raw = blocks[i].textContent;
        blocks[i].innerHTML = Prism.highlight(raw, Prism.languages.kolang, 'kolang');
        blocks[i].setAttribute('data-highlighted', 'kolang');
      } catch (e) {
        if (window.console) console.warn('kolang highlight failed:', e);
      }
    }
  }

  function loadPrism() {
    if (window.Prism) { defineKolang(); highlightAll(); return; }

    var s = document.createElement('script');
    s.src = 'https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/prism.min.js';
    s.async = true;
    s.onload = function () { defineKolang(); highlightAll(); };
    s.onerror = function () {
      // CDN در دسترس نیست — spanهای دستی + CSS رنگی همچنان کار می‌کنند.
      if (window.console) console.warn('Prism CDN در دسترس نیست؛ از هایلایت دستی استفاده می‌شود.');
    };
    document.head.appendChild(s);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', loadPrism);
  } else {
    loadPrism();
  }
})();
