/**
 * Markdown skill editor — CodeMirror 5 + optional preview (marked).
 */
(function () {
  let cm = null;
  let previewEl = null;
  let editPane = null;
  let onChangeCb = null;
  let suppressChange = false;

  function ensureInit() {
    if (cm) return;
    const wrap = document.getElementById("skills-editor-wrap");
    previewEl = document.getElementById("skills-preview");
    editPane = document.getElementById("skills-editor-pane");
    if (!wrap || typeof CodeMirror === "undefined") return;

    cm = CodeMirror(wrap, {
      value: "",
      mode: "markdown",
      theme: "sdast-light",
      lineNumbers: true,
      lineWrapping: true,
      tabSize: 2,
      indentUnit: 2,
      readOnly: true,
      viewportMargin: Infinity,
    });

    cm.on("change", () => {
      if (suppressChange) return;
      if (onChangeCb) onChangeCb();
      updatePreview();
    });
  }

  function updatePreview() {
    if (!previewEl || !cm) return;
    const md = cm.getValue();
    if (typeof marked !== "undefined") {
      previewEl.innerHTML = marked.parse(md, { breaks: true, gfm: true });
    } else {
      previewEl.textContent = md;
    }
  }

  function setTab(tab) {
    const isPreview = tab === "preview";
    if (editPane) editPane.classList.toggle("hidden", isPreview);
    if (previewEl) previewEl.classList.toggle("hidden", !isPreview);
    document.querySelectorAll(".skills-tab-btn").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.tab === tab);
    });
    if (isPreview) updatePreview();
    if (!isPreview && cm) cm.refresh();
  }

  window.SkillsEditor = {
    init() {
      ensureInit();
      document.querySelectorAll(".skills-tab-btn").forEach((btn) => {
        btn.addEventListener("click", () => setTab(btn.dataset.tab));
      });
    },

    onChange(fn) {
      onChangeCb = fn;
    },

    getValue() {
      ensureInit();
      return cm ? cm.getValue() : "";
    },

    setValue(text) {
      ensureInit();
      if (cm) {
        suppressChange = true;
        cm.setValue(text || "");
        cm.refresh();
        suppressChange = false;
      }
      updatePreview();
    },

    setReadOnly(ro) {
      ensureInit();
      if (cm) cm.setOption("readOnly", ro);
    },

    focus() {
      ensureInit();
      if (cm) {
        setTab("edit");
        cm.focus();
      }
    },

    refresh() {
      if (cm) cm.refresh();
    },
  };
})();
