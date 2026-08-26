# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-08-26

### Added

- Initial release
- Full tree-walking interpreter in Go
- Persian syntax: verb-final verbs, ezafe member access, «» strings, Persian digits
- Arithmetic: × ÷ * ÷/ % (multiply, divide, power, floor-div, modulo)
- Control flow: اگر/وگرنه (if/else), برای (range/in), تاوقتی (while), اتمام/بروبعدی (break/continue)
- Data types: صحیح (int), اعشاری (float), متن (string), بولی (bool), فهرست (list), گنجه (dict), قفسه (stack), مجموعه (set), تهی (none)
- OOP: گونه (class), وارث (inheritance), خود (self), والد (super), رابط (interface), رهی (implements)
- Exceptions: بپا/بگیر/درنهایت (try/catch/finally), خطا (error) factory, custom exceptions (وارث استثنا)
- Generators: ای بساز (yield), بساز‌از (yield-from)
- Decorators: پوشش
- Comprehensions: list/dict/set/genexp with filters
- Pipes: |>
- Ternary: X اگر cond باشد وگرنه Y
- Concurrency: برو (goroutines), کانال (channels), << >>, ببند (close), بسته‌استِ (is-closed)
- Defer: تأخیری (postfix)
- Typing: gradual (runtime-checked annotations)
- Scope: جهانی (global), نامحلی (nonlocal)
- Varargs: *args, **kwargs
- With statement: با ... بانام
- 13+ builtins: بنویس, بگیر, طول, نوع, بازه, جمع, کمینه, بیشینه, مرتب, مطلق, گرد, معکوس, شمارش, بقچه, نگاشت, پالایش, بولی, هویت, اجرا, ...
- ریاضی (math) standard module
- Persian error messages
- 32 example programs
- 390 tests (race-clean)