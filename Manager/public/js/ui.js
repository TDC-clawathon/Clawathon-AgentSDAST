/* ============================================================
   AgentSDAST shared UI component system (vanilla, zero-dependency).
   Mirrors components/ui/: Modal · ConfirmDialog · Toast · Dropdown ·
   LoadingOverlay · EmptyState. Matches the dashboard design language
   (angular HUD, metallic panels, cyan/magenta accents, shared tokens).

   Public API (window.UI):
     UI.modal.open({...}) -> { close, promise, els }
     UI.confirmDialog.open({...}) -> Promise<boolean>
     UI.confirmDialog.prompt({...}) -> Promise<string|null>
     UI.toast.success/error/info/warning(msg, opts)
     UI.Dropdown.enhance(selectEl) / .enhanceAll(selector)
     UI.LoadingOverlay.show(target?, msg) / .hide(handle)
     UI.EmptyState.html({icon,title,message})
   ============================================================ */
(function () {
  const UI = (window.UI = window.UI || {});

  function el(tag, cls, html) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (html != null) e.innerHTML = html;
    return e;
  }
  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  /* ---------------- Modal (base) ---------------- */
  const modalStack = [];

  function openModal(opts) {
    const {
      title = "",
      bodyNode = null,
      bodyHtml = null,
      actions = [], // [{ label, variant, value, keepOpen, autofocus }]
      danger = false,
      dismissible = true,
      dismissValue = null,
      className = "",
      onResult = null,
    } = opts || {};

    const prevFocus = document.activeElement;
    const overlay = el("div", "ui-modal-overlay");
    const modal = el("div", "ui-modal" + (danger ? " ui-modal--danger" : "") + (className ? " " + className : ""));
    modal.setAttribute("role", "dialog");
    modal.setAttribute("aria-modal", "true");

    const head = el("div", "ui-modal__head");
    head.appendChild(el("h3", "ui-modal__title", esc(title)));
    const closeX = el("button", "ui-modal__close", "&times;");
    closeX.type = "button";
    closeX.setAttribute("aria-label", "Close");
    head.appendChild(closeX);

    const body = el("div", "ui-modal__body");
    if (bodyNode) body.appendChild(bodyNode);
    else if (bodyHtml != null) body.innerHTML = bodyHtml;

    modal.appendChild(head);
    modal.appendChild(body);

    let resolveFn;
    const promise = new Promise((r) => (resolveFn = r));
    let closed = false;

    function close(value) {
      if (closed) return;
      closed = true;
      overlay.classList.remove("open");
      document.removeEventListener("keydown", onKey, true);
      const idx = modalStack.indexOf(api);
      if (idx >= 0) modalStack.splice(idx, 1);
      setTimeout(() => overlay.remove(), 200);
      if (prevFocus && prevFocus.focus) {
        try { prevFocus.focus(); } catch (_) {}
      }
      if (onResult) onResult(value);
      resolveFn(value);
    }

    const api = { overlay, modal, body, close, promise };

    if (actions.length) {
      const foot = el("div", "ui-modal__foot");
      actions.forEach((a) => {
        const b = el("button", "btn ui-modal__action " + (a.variant || "btn-ghost"));
        b.type = "button";
        b.textContent = a.label;
        b.addEventListener("click", () => {
          if (a.onClick && a.onClick(api) === false) return;
          if (!a.keepOpen) close(a.value);
        });
        a._btn = b;
        foot.appendChild(b);
      });
      modal.appendChild(foot);
    }

    closeX.addEventListener("click", () => close(dismissValue));
    if (dismissible) {
      overlay.addEventListener("click", (e) => {
        if (e.target === overlay) close(dismissValue);
      });
    }

    function onKey(e) {
      if (modalStack[modalStack.length - 1] !== api) return;
      if (e.key === "Escape" && dismissible) {
        e.preventDefault();
        close(dismissValue);
      }
    }
    document.addEventListener("keydown", onKey, true);

    overlay.appendChild(modal);
    document.body.appendChild(overlay);
    modalStack.push(api);

    // animate in
    requestAnimationFrame(() => overlay.classList.add("open"));

    // focus: autofocus action, else first input, else close button
    setTimeout(() => {
      const auto = actions.find((a) => a.autofocus);
      const target =
        (auto && auto._btn) || body.querySelector("input,textarea,select,button") || closeX;
      if (target && target.focus) target.focus();
    }, 30);

    return api;
  }

  UI.modal = { open: openModal };

  /* ---------------- ConfirmDialog ---------------- */
  UI.confirmDialog = {
    open(opts) {
      const {
        title = "Are you sure?",
        message = "",
        detail = null, // secondary line (e.g. a scan id)
        confirmText = "Confirm",
        cancelText = "Cancel",
        danger = false,
        dismissible = true,
      } = opts || {};

      const body = el("div");
      if (message) body.appendChild(el("p", "ui-modal__message", esc(message)));
      if (detail) body.appendChild(el("div", "ui-modal__detail", esc(detail)));

      return openModal({
        title,
        bodyNode: body,
        danger,
        dismissible,
        dismissValue: false,
        actions: [
          { label: cancelText, variant: "btn-ghost", value: false },
          {
            label: confirmText,
            variant: danger ? "btn-danger" : "btn-primary",
            value: true,
            autofocus: true,
          },
        ],
      }).promise;
    },

    prompt(opts) {
      const {
        title = "Enter a value",
        message = "",
        placeholder = "",
        value = "",
        confirmText = "OK",
        cancelText = "Cancel",
      } = opts || {};

      const body = el("div");
      if (message) body.appendChild(el("p", "ui-modal__message", esc(message)));
      const input = el("input", "url-input ui-modal__input");
      input.type = "text";
      input.placeholder = placeholder;
      input.value = value;
      body.appendChild(input);

      const api = openModal({
        title,
        bodyNode: body,
        dismissible: true,
        dismissValue: null,
        actions: [
          { label: cancelText, variant: "btn-ghost", value: null },
          {
            label: confirmText,
            variant: "btn-primary",
            keepOpen: true,
            onClick: () => {
              const v = input.value.trim();
              api.close(v || null);
            },
          },
        ],
      });
      input.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          api.close(input.value.trim() || null);
        }
      });
      setTimeout(() => input.focus(), 40);
      return api.promise;
    },
  };

  /* ---------------- Toast ---------------- */
  function toastContainer() {
    let c = document.getElementById("toast-container");
    if (!c) {
      c = el("div", "toast-container");
      c.id = "toast-container";
      document.body.appendChild(c);
    }
    return c;
  }

  function showToastInternal(message, opts) {
    const { type = "success", duration, details = null } = opts || {};
    const c = toastContainer();
    const t = el("div", `toast ${type}`);

    const row = el("div", "toast__row");
    row.appendChild(el("span", "toast__msg", esc(message)));
    const closeBtn = el("button", "toast__close", "&times;");
    closeBtn.type = "button";
    closeBtn.setAttribute("aria-label", "Dismiss");
    row.appendChild(closeBtn);
    t.appendChild(row);

    if (details) {
      const det = el("details", "toast__details");
      det.appendChild(el("summary", null, "Details"));
      const pre = el("pre", "toast__pre");
      pre.textContent = typeof details === "string" ? details : JSON.stringify(details, null, 2);
      det.appendChild(pre);
      const copy = el("button", "btn btn-ghost btn-sm toast__copy", "Copy details");
      copy.type = "button";
      copy.addEventListener("click", () => {
        const text = pre.textContent;
        if (navigator.clipboard) navigator.clipboard.writeText(text).catch(() => {});
        copy.textContent = "Copied";
        setTimeout(() => (copy.textContent = "Copy details"), 1500);
      });
      det.appendChild(copy);
      t.appendChild(det);
    }

    let removed = false;
    function remove() {
      if (removed) return;
      removed = true;
      t.classList.add("toast--out");
      setTimeout(() => t.remove(), 220);
    }
    closeBtn.addEventListener("click", remove);
    c.appendChild(t);
    requestAnimationFrame(() => t.classList.add("toast--in"));

    // Errors with technical details stay until dismissed; others auto-dismiss.
    const ms = duration != null ? duration : details ? 0 : type === "error" ? 6000 : 4000;
    if (ms > 0) setTimeout(remove, ms);
    return { dismiss: remove };
  }

  UI.toast = {
    show: showToastInternal,
    success: (m, o) => showToastInternal(m, { ...(o || {}), type: "success" }),
    error: (m, o) => showToastInternal(m, { ...(o || {}), type: "error" }),
    info: (m, o) => showToastInternal(m, { ...(o || {}), type: "info" }),
    warning: (m, o) => showToastInternal(m, { ...(o || {}), type: "warning" }),
  };

  /* ---------------- Dropdown (custom <select>) ---------------- */
  UI.Dropdown = {
    // Progressive enhancement: hides the native <select> (kept as source of
    // truth so existing .value reads + change events still work) and renders a
    // styled, keyboard-accessible listbox over it. Idempotent (re-call to refresh).
    enhance(select) {
      if (!select) return null;
      if (select.__uiDropdown) {
        select.__uiDropdown.refresh();
        return select.__uiDropdown;
      }

      const wrap = el("div", "ui-select");
      select.parentNode.insertBefore(wrap, select);
      wrap.appendChild(select);
      select.classList.add("ui-select__native");
      select.setAttribute("aria-hidden", "true");
      select.tabIndex = -1;

      const btn = el("button", "ui-select__button");
      btn.type = "button";
      btn.setAttribute("aria-haspopup", "listbox");
      btn.setAttribute("aria-expanded", "false");
      const label = el("span", "ui-select__label");
      btn.appendChild(label);
      btn.appendChild(el("span", "ui-select__caret", "▼"));
      const menu = el("div", "ui-select__menu");
      menu.setAttribute("role", "listbox");
      wrap.appendChild(btn);
      wrap.appendChild(menu);

      let isOpen = false;
      let activeIndex = -1;
      let items = [];

      function currentText() {
        const o = select.options[select.selectedIndex];
        return o ? o.textContent : "";
      }
      function refresh() {
        label.textContent = currentText() || select.dataset.placeholder || "Select…";
        btn.disabled = select.disabled;
        wrap.classList.toggle("is-disabled", select.disabled);
        menu.innerHTML = "";
        items = Array.from(select.options).map((o, i) => {
          const it = el("div", "ui-select__option");
          it.textContent = o.textContent;
          it.setAttribute("role", "option");
          it.setAttribute("aria-selected", String(i === select.selectedIndex));
          if (i === select.selectedIndex) it.classList.add("is-selected");
          if (o.disabled) it.classList.add("is-disabled");
          it.addEventListener("click", () => {
            if (o.disabled) return;
            choose(i);
            closeMenu();
            btn.focus();
          });
          menu.appendChild(it);
          return it;
        });
      }
      function choose(i) {
        if (i < 0 || i >= select.options.length) return;
        select.selectedIndex = i;
        select.dispatchEvent(new Event("change", { bubbles: true }));
        refresh();
      }
      function highlight() {
        items.forEach((it, i) => it.classList.toggle("is-active", i === activeIndex));
        if (items[activeIndex]) items[activeIndex].scrollIntoView({ block: "nearest" });
      }

      // Adaptive positioning: the menu is portaled to <body> (position: fixed) so
      // no ancestor (overflow:hidden / clip-path / scroll panel / z-index) can clip
      // it. It opens downward when there's room, otherwise upward; if neither side
      // fully fits it picks the larger side and caps height with scroll.
      const GAP = 4;
      const MAX_MENU = 320;
      function position() {
        const rect = btn.getBoundingClientRect();
        const vh = window.innerHeight;
        const spaceBelow = vh - rect.bottom - GAP;
        const spaceAbove = rect.top - GAP;
        menu.style.maxHeight = "none";
        const desired = Math.min(menu.scrollHeight, MAX_MENU);

        let placement, maxH;
        if (desired <= spaceBelow) {
          placement = "bottom";
          maxH = spaceBelow;
        } else if (desired <= spaceAbove) {
          placement = "top";
          maxH = spaceAbove;
        } else if (spaceBelow >= spaceAbove) {
          placement = "bottom";
          maxH = spaceBelow;
        } else {
          placement = "top";
          maxH = spaceAbove;
        }

        menu.style.left = Math.round(rect.left) + "px";
        menu.style.width = Math.round(rect.width) + "px";
        menu.style.maxHeight = Math.max(120, Math.min(MAX_MENU, maxH)) + "px";
        if (placement === "bottom") {
          menu.style.top = Math.round(rect.bottom + GAP) + "px";
          menu.style.bottom = "auto";
        } else {
          menu.style.top = "auto";
          menu.style.bottom = Math.round(vh - rect.top + GAP) + "px";
        }
        menu.classList.toggle("is-top", placement === "top");
        menu.classList.toggle("is-bottom", placement === "bottom");
      }

      function openMenu() {
        if (btn.disabled || isOpen || !items.length) return;
        isOpen = true;
        wrap.classList.add("open");
        btn.setAttribute("aria-expanded", "true");
        // Portal to body so the menu floats above everything, unclipped.
        document.body.appendChild(menu);
        menu.classList.add("ui-select__menu--open");
        position();
        requestAnimationFrame(() => menu.classList.add("is-visible"));
        activeIndex = select.selectedIndex >= 0 ? select.selectedIndex : 0;
        highlight();
        document.addEventListener("click", onDoc, true);
        // Reposition (don't clip) on resize/scroll while open — handles small
        // screens, zoom, and scrolling inside any ancestor (capture = true).
        window.addEventListener("resize", position);
        window.addEventListener("scroll", position, true);
      }
      function closeMenu() {
        if (!isOpen) return;
        isOpen = false;
        wrap.classList.remove("open");
        btn.setAttribute("aria-expanded", "false");
        menu.classList.remove("is-visible");
        document.removeEventListener("click", onDoc, true);
        window.removeEventListener("resize", position);
        window.removeEventListener("scroll", position, true);
        const m = menu;
        setTimeout(() => {
          if (!isOpen) m.classList.remove("ui-select__menu--open");
        }, 160);
      }
      function onDoc(e) {
        if (!wrap.contains(e.target) && !menu.contains(e.target)) closeMenu();
      }

      btn.addEventListener("click", () => (isOpen ? closeMenu() : openMenu()));
      btn.addEventListener("keydown", (e) => {
        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          if (!isOpen) return openMenu();
          activeIndex = Math.max(0, Math.min(items.length - 1, activeIndex + (e.key === "ArrowDown" ? 1 : -1)));
          highlight();
        } else if (e.key === "Home") {
          if (isOpen) { e.preventDefault(); activeIndex = 0; highlight(); }
        } else if (e.key === "End") {
          if (isOpen) { e.preventDefault(); activeIndex = items.length - 1; highlight(); }
        } else if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          if (isOpen && activeIndex >= 0) { choose(activeIndex); closeMenu(); }
          else openMenu();
        } else if (e.key === "Escape") {
          if (isOpen) { e.preventDefault(); closeMenu(); }
        } else if (e.key === "Tab") {
          closeMenu();
        } else if (e.key.length === 1) {
          const ch = e.key.toLowerCase();
          const start = activeIndex + 1;
          for (let k = 0; k < items.length; k++) {
            const idx = (start + k) % items.length;
            if ((select.options[idx].textContent || "").toLowerCase().startsWith(ch)) {
              activeIndex = idx;
              if (!isOpen) openMenu();
              highlight();
              break;
            }
          }
        }
      });

      const ctrl = {
        refresh,
        el: wrap,
        setLoading(b) {
          wrap.classList.toggle("is-loading", !!b);
          btn.disabled = !!b || select.disabled;
        },
        setDisabled(b) {
          select.disabled = !!b;
          refresh();
        },
        open: openMenu,
        close: closeMenu,
      };
      select.__uiDropdown = ctrl;
      refresh();
      return ctrl;
    },

    enhanceAll(selector) {
      document.querySelectorAll(selector).forEach((s) => UI.Dropdown.enhance(s));
    },
  };

  /* ---------------- LoadingOverlay ---------------- */
  UI.LoadingOverlay = {
    show(target, message) {
      const host = target || document.body;
      if (getComputedStyle(host).position === "static" && host !== document.body) {
        host.style.position = "relative";
      }
      const ov = el("div", "ui-loading-overlay");
      ov.innerHTML = `<div class="ui-loading"><span class="btn-spinner"></span><span>${esc(message || "Loading…")}</span></div>`;
      host.appendChild(ov);
      requestAnimationFrame(() => ov.classList.add("open"));
      return { hide: () => { ov.classList.remove("open"); setTimeout(() => ov.remove(), 200); } };
    },
    hide(handle) {
      if (handle && handle.hide) handle.hide();
    },
  };

  /* ---------------- EmptyState ---------------- */
  UI.EmptyState = {
    html({ icon = "", title = "", message = "" } = {}) {
      return `<div class="ui-empty">
        ${icon ? `<div class="ui-empty__icon">${icon}</div>` : ""}
        ${title ? `<div class="ui-empty__title">${esc(title)}</div>` : ""}
        ${message ? `<div class="ui-empty__message">${esc(message)}</div>` : ""}
      </div>`;
    },
  };
})();
