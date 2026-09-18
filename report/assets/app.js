(function () {
  const dataElement = document.getElementById("report-data");
  if (!dataElement) return;

  let report = {};
  try {
    report = JSON.parse(dataElement.textContent || "{}");
  } catch (e) {
    console.error("Failed to parse report data:", e);
    return;
  }

  const files = report.files || [];
  let activeIndex = 0;
  let viewMode = "split"; // 'split' | 'unified'
  let filterStatus = "all"; // 'all' | 'changed' | 'added' | 'modified' | 'deleted' | 'identical'
  let searchQuery = "";

  const fileListEl = document.getElementById("file-list");
  const searchInputEl = document.getElementById("search-input");
  const activeFileTitleEl = document.getElementById("active-file-title");
  const diffContainerEl = document.getElementById("diff-container");
  const filterPills = document.querySelectorAll(".pill-btn");
  const btnSplit = document.getElementById("btn-split");
  const btnUnified = document.getElementById("btn-unified");
  const btnPrevFile = document.getElementById("btn-prev-file");
  const btnNextFile = document.getElementById("btn-next-file");
  const fileCounterEl = document.getElementById("file-counter");

  function escapeHtml(text) {
    if (!text) return "";
    return text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  function formatBytes(bytes) {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  }

  function getStatusClass(status) {
    switch (status) {
      case "ADDED": return "added";
      case "DELETED": return "deleted";
      case "MODIFIED": return "modified";
      case "BINARY": return "binary";
      case "IDENTICAL": return "identical";
      default: return "";
    }
  }

  function getFilteredFiles() {
    return files.map((file, originalIndex) => ({ file, originalIndex })).filter(({ file }) => {
      // Status filter
      if (filterStatus === "changed" && file.status === "IDENTICAL") return false;
      if (filterStatus === "added" && file.status !== "ADDED") return false;
      if (filterStatus === "modified" && file.status !== "MODIFIED") return false;
      if (filterStatus === "deleted" && file.status !== "DELETED") return false;
      if (filterStatus === "identical" && file.status !== "IDENTICAL") return false;

      // Search query
      if (searchQuery.trim() !== "") {
        const query = searchQuery.toLowerCase();
        if (!file.relativePath.toLowerCase().includes(query)) {
          return false;
        }
      }

      return true;
    });
  }

  function updateNavControls() {
    if (!btnPrevFile || !btnNextFile || !fileCounterEl) return;
    const filtered = getFilteredFiles();
    if (filtered.length === 0) {
      fileCounterEl.textContent = "0 / 0";
      btnPrevFile.disabled = true;
      btnNextFile.disabled = true;
      return;
    }

    const currentFilteredIdx = filtered.findIndex(item => item.originalIndex === activeIndex);
    if (currentFilteredIdx === -1) {
      fileCounterEl.textContent = `- / ${filtered.length}`;
      btnPrevFile.disabled = true;
      btnNextFile.disabled = false;
    } else {
      fileCounterEl.textContent = `${currentFilteredIdx + 1} / ${filtered.length}`;
      btnPrevFile.disabled = currentFilteredIdx <= 0;
      btnNextFile.disabled = currentFilteredIdx >= filtered.length - 1;
    }
  }

  function navigateFile(step) {
    const filtered = getFilteredFiles();
    if (filtered.length === 0) return;

    let currentFilteredIdx = filtered.findIndex(item => item.originalIndex === activeIndex);
    let nextFilteredIdx;

    if (currentFilteredIdx === -1) {
      nextFilteredIdx = step > 0 ? 0 : filtered.length - 1;
    } else {
      nextFilteredIdx = currentFilteredIdx + step;
    }

    if (nextFilteredIdx < 0) nextFilteredIdx = 0;
    if (nextFilteredIdx >= filtered.length) nextFilteredIdx = filtered.length - 1;

    if (currentFilteredIdx !== nextFilteredIdx || currentFilteredIdx === -1) {
      activeIndex = filtered[nextFilteredIdx].originalIndex;
      renderSidebar();
      renderDiff();
    }
  }

  function renderSidebar() {
    const filtered = getFilteredFiles();
    fileListEl.innerHTML = "";

    if (filtered.length === 0) {
      fileListEl.innerHTML = `<li style="padding: 16px; color: var(--text-muted); text-align: center;">No matching files</li>`;
      updateNavControls();
      return;
    }

    let activeLi = null;

    filtered.forEach(({ file, originalIndex }) => {
      const li = document.createElement("li");
      const isActive = originalIndex === activeIndex;
      li.className = `file-item ${isActive ? "active" : ""}`;
      li.title = file.relativePath;
      if (isActive) {
        activeLi = li;
      }
      
      const statusClass = getStatusClass(file.status);
      let statsHtml = "";
      if (file.status === "MODIFIED" || file.status === "ADDED" || file.status === "DELETED") {
        if (file.additions > 0 || file.deletions > 0) {
          statsHtml = `
            <div class="file-stats">
              ${file.additions > 0 ? `<span class="stat-add">+${file.additions}</span>` : ""}
              ${file.deletions > 0 ? `<span class="stat-del">-${file.deletions}</span>` : ""}
            </div>
          `;
        }
      }

      li.innerHTML = `
        <div class="file-info" title="${escapeHtml(file.relativePath)}">
          <span class="status-badge ${statusClass}">${escapeHtml(file.status)}</span>
          <span class="file-path" title="${escapeHtml(file.relativePath)}">${escapeHtml(file.relativePath)}</span>
        </div>
        ${statsHtml}
      `;

      li.addEventListener("click", () => {
        activeIndex = originalIndex;
        renderSidebar();
        renderDiff();
      });

      fileListEl.appendChild(li);
    });

    if (activeLi) {
      activeLi.scrollIntoView({ block: "nearest" });
    }

    updateNavControls();
  }

  function tokenize(str) {
    if (!str) return [];
    const regex = /[a-zA-Z0-9_]+|[^\s\w]|\s+/g;
    const tokens = [];
    let match;
    while ((match = regex.exec(str)) !== null) {
      tokens.push(match[0]);
    }
    return tokens;
  }

  function computeTokenDiff(oldStr, newStr) {
    if (!oldStr && !newStr) return { leftHtml: "", rightHtml: "" };
    if (!oldStr) return { leftHtml: "", rightHtml: `<span class="diff-highlight-add">${escapeHtml(newStr)}</span>` };
    if (!newStr) return { leftHtml: `<span class="diff-highlight-del">${escapeHtml(oldStr)}</span>`, rightHtml: "" };

    const a = tokenize(oldStr);
    const b = tokenize(newStr);
    const N = a.length;
    const M = b.length;

    // Fast check for exact equality
    if (oldStr === newStr) {
      const esc = escapeHtml(oldStr);
      return { leftHtml: esc, rightHtml: esc };
    }

    // Limit Myers diff size to avoid slow rendering on pathological single-line extremes
    if (N + M > 400) {
      return {
        leftHtml: escapeHtml(oldStr),
        rightHtml: escapeHtml(newStr)
      };
    }

    const MAX = N + M;
    const v = new Map();
    v.set(1, 0);
    const trace = [];

    for (let d = 0; d <= MAX; d++) {
      trace.push(new Map(v));
      let found = false;
      for (let k = -d; k <= d; k += 2) {
        let x;
        if (k === -d || (k !== d && (v.get(k - 1) ?? 0) < (v.get(k + 1) ?? 0))) {
          x = v.get(k + 1) ?? 0;
        } else {
          x = (v.get(k - 1) ?? 0) + 1;
        }
        let y = x - k;

        while (x < N && y < M && a[x] === b[y]) {
          x++;
          y++;
        }
        v.set(k, x);
        if (x >= N && y >= M) {
          found = true;
          break;
        }
      }
      if (found) break;
    }

    let x = N;
    let y = M;
    const leftSpans = [];
    const rightSpans = [];

    for (let d = trace.length - 1; d >= 0; d--) {
      const vPrev = trace[d];
      const k = x - y;
      let prevK;
      if (k === -d || (k !== d && (vPrev.get(k - 1) ?? 0) < (vPrev.get(k + 1) ?? 0))) {
        prevK = k + 1;
      } else {
        prevK = k - 1;
      }

      const prevX = vPrev.get(prevK) ?? 0;
      const prevY = prevX - prevK;

      while (x > prevX && y > prevY) {
        x--;
        y--;
        leftSpans.unshift({ type: "equal", text: a[x] });
        rightSpans.unshift({ type: "equal", text: b[y] });
      }

      if (d > 0) {
        if (x > prevX) {
          x--;
          leftSpans.unshift({ type: "delete", text: a[x] });
        } else if (y > prevY) {
          y--;
          rightSpans.unshift({ type: "insert", text: b[y] });
        }
      }
    }

    function renderSpans(spans, highlightClass) {
      return spans
        .map((s) => {
          const esc = escapeHtml(s.text);
          if (s.type === "equal") return esc;
          return `<span class="${highlightClass}">${esc}</span>`;
        })
        .join("");
    }

    return {
      leftHtml: renderSpans(leftSpans, "diff-highlight-del"),
      rightHtml: renderSpans(rightSpans, "diff-highlight-add")
    };
  }

  function buildSplitRows(lines) {
    const rows = [];
    let i = 0;
    while (i < lines.length) {
      const line = lines[i];

      if (line.type === "EQUAL") {
        rows.push({
          left: { num: line.leftLineNum, content: line.content, type: "equal" },
          right: { num: line.rightLineNum, content: line.content, type: "equal" },
        });
        i++;
      } else {
        // Collect deletions and insertions chunk
        const deletes = [];
        const inserts = [];

        while (i < lines.length && lines[i].type === "DELETE") {
          deletes.push(lines[i]);
          i++;
        }
        while (i < lines.length && lines[i].type === "INSERT") {
          inserts.push(lines[i]);
          i++;
        }

        const maxLen = Math.max(deletes.length, inserts.length);
        for (let j = 0; j < maxLen; j++) {
          const del = deletes[j] || null;
          const ins = inserts[j] || null;

          rows.push({
            left: del ? { num: del.leftLineNum, content: del.content, type: "delete" } : null,
            right: ins ? { num: ins.rightLineNum, content: ins.content, type: "insert" } : null,
          });
        }
      }
    }
    return rows;
  }

  function renderDiff() {
    if (files.length === 0 || activeIndex < 0 || activeIndex >= files.length) {
      activeFileTitleEl.innerHTML = "No file selected";
      activeFileTitleEl.title = "";
      diffContainerEl.innerHTML = `<div class="empty-state"><p>No comparison data available</p></div>`;
      updateNavControls();
      return;
    }

    const file = files[activeIndex];
    const statusClass = getStatusClass(file.status);

    activeFileTitleEl.title = file.relativePath;
    activeFileTitleEl.innerHTML = `
      <span class="status-badge ${statusClass}">${escapeHtml(file.status)}</span>
      <span>${escapeHtml(file.relativePath)}</span>
      ${file.additions > 0 ? `<span class="stat-add" style="font-size:0.8rem; margin-left:8px;">+${file.additions}</span>` : ""}
      ${file.deletions > 0 ? `<span class="stat-del" style="font-size:0.8rem;">-${file.deletions}</span>` : ""}
    `;

    if (file.isBinary) {
      diffContainerEl.innerHTML = `
        <div class="empty-state">
          <p><strong>Binary file differ</strong></p>
          <p style="margin-top: 8px; font-size: 0.85rem;">
            Left size: ${formatBytes(file.leftSize)} &bull; Right size: ${formatBytes(file.rightSize)}
          </p>
          <p style="margin-top: 4px; font-size: 0.78rem; color: var(--text-muted);">
            Binary contents cannot be displayed as a text diff.
          </p>
        </div>
      `;
      updateNavControls();
      return;
    }

    if (file.status === "IDENTICAL") {
      diffContainerEl.innerHTML = `
        <div class="empty-state">
          <p>Files are identical (${formatBytes(file.leftSize)})</p>
        </div>
      `;
      updateNavControls();
      return;
    }

    const lines = file.lines || [];
    if (lines.length === 0) {
      diffContainerEl.innerHTML = `
        <div class="empty-state">
          <p>File is empty</p>
        </div>
      `;
      updateNavControls();
      return;
    }

    if (viewMode === "split") {
      const rows = buildSplitRows(lines);
      let html = `
        <table class="diff-table split-view-table">
          <colgroup>
            <col style="width: 48px;">
            <col style="width: calc(50% - 48px);">
            <col style="width: 48px;">
            <col style="width: calc(50% - 48px);">
          </colgroup>
          <thead class="split-header-row">
            <tr>
              <th colspan="2">Left (Original)</th>
              <th colspan="2">Right (Modified)</th>
            </tr>
          </thead>
          <tbody>
      `;

      rows.forEach((row) => {
        const leftClass = row.left ? (row.left.type === "delete" ? "delete" : "") : "empty-cell";
        const rightClass = row.right ? (row.right.type === "insert" ? "insert" : "") : "empty-cell";

        let leftContent = "";
        let rightContent = "";

        if (row.left && row.right && row.left.type === "delete" && row.right.type === "insert") {
          const diffRes = computeTokenDiff(row.left.content, row.right.content);
          leftContent = diffRes.leftHtml;
          rightContent = diffRes.rightHtml;
        } else {
          leftContent = row.left ? escapeHtml(row.left.content) : "";
          rightContent = row.right ? escapeHtml(row.right.content) : "";
        }

        const leftNum = row.left ? row.left.num : "";
        const rightNum = row.right ? row.right.num : "";

        html += `
          <tr class="diff-row">
            <td class="line-num ${leftClass}">${leftNum}</td>
            <td class="line-code ${leftClass}">${leftContent}</td>
            <td class="line-num ${rightClass}">${rightNum}</td>
            <td class="line-code ${rightClass}">${rightContent}</td>
          </tr>
        `;
      });

      html += `</tbody></table>`;
      diffContainerEl.innerHTML = html;
    } else {
      // Unified View
      let html = `
        <table class="diff-table">
          <colgroup>
            <col style="width: 44px;">
            <col style="width: 44px;">
            <col style="width: 24px;">
            <col style="width: auto;">
          </colgroup>
          <tbody>
      `;

      let i = 0;
      while (i < lines.length) {
        const line = lines[i];
        if (line.type === "EQUAL") {
          html += `
            <tr class="diff-row">
              <td class="line-num">${line.leftLineNum}</td>
              <td class="line-num">${line.rightLineNum}</td>
              <td style="text-align: center; color: var(--text-muted); user-select: none;"> </td>
              <td class="line-code">${escapeHtml(line.content)}</td>
            </tr>
          `;
          i++;
        } else {
          const deletes = [];
          const inserts = [];
          while (i < lines.length && lines[i].type === "DELETE") {
            deletes.push(lines[i]);
            i++;
          }
          while (i < lines.length && lines[i].type === "INSERT") {
            inserts.push(lines[i]);
            i++;
          }

          const pairedCount = Math.min(deletes.length, inserts.length);

          for (let d = 0; d < deletes.length; d++) {
            const del = deletes[d];
            let delHtml = escapeHtml(del.content);
            if (d < pairedCount) {
              const diffRes = computeTokenDiff(del.content, inserts[d].content);
              delHtml = diffRes.leftHtml;
            }
            html += `
              <tr class="diff-row delete">
                <td class="line-num">${del.leftLineNum}</td>
                <td class="line-num"></td>
                <td style="text-align: center; color: var(--deleted-text); user-select: none;">-</td>
                <td class="line-code delete">${delHtml}</td>
              </tr>
            `;
          }

          for (let insIdx = 0; insIdx < inserts.length; insIdx++) {
            const ins = inserts[insIdx];
            let insHtml = escapeHtml(ins.content);
            if (insIdx < pairedCount) {
              const diffRes = computeTokenDiff(deletes[insIdx].content, ins.content);
              insHtml = diffRes.rightHtml;
            }
            html += `
              <tr class="diff-row insert">
                <td class="line-num"></td>
                <td class="line-num">${ins.rightLineNum}</td>
                <td style="text-align: center; color: var(--added-text); user-select: none;">+</td>
                <td class="line-code insert">${insHtml}</td>
              </tr>
            `;
          }
        }
      }

      html += `</tbody></table>`;
      diffContainerEl.innerHTML = html;
    }

    updateNavControls();
  }

  // Setup Event Handlers
  if (btnPrevFile) {
    btnPrevFile.addEventListener("click", () => navigateFile(-1));
  }

  if (btnNextFile) {
    btnNextFile.addEventListener("click", () => navigateFile(1));
  }

  window.addEventListener("keydown", (e) => {
    const target = e.target;
    const isInput = target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable);

    if (isInput) {
      if (e.key === "Escape") {
        target.blur();
      }
      return;
    }

    if (e.key === "ArrowDown" || e.key === "ArrowRight" || e.key === "j" || e.key === "J") {
      e.preventDefault();
      navigateFile(1);
    } else if (e.key === "ArrowUp" || e.key === "ArrowLeft" || e.key === "k" || e.key === "K") {
      e.preventDefault();
      navigateFile(-1);
    }
  });

  searchInputEl.addEventListener("input", (e) => {
    searchQuery = e.target.value;
    const filtered = getFilteredFiles();
    if (filtered.length > 0 && !filtered.some(item => item.originalIndex === activeIndex)) {
      activeIndex = filtered[0].originalIndex;
    }
    renderSidebar();
    renderDiff();
  });

  filterPills.forEach((btn) => {
    btn.addEventListener("click", () => {
      filterPills.forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      filterStatus = btn.getAttribute("data-filter") || "all";
      const filtered = getFilteredFiles();
      if (filtered.length > 0 && !filtered.some(item => item.originalIndex === activeIndex)) {
        activeIndex = filtered[0].originalIndex;
      }
      renderSidebar();
      renderDiff();
    });
  });

  btnSplit.addEventListener("click", () => {
    viewMode = "split";
    btnSplit.classList.add("active");
    btnUnified.classList.remove("active");
    renderDiff();
  });

  btnUnified.addEventListener("click", () => {
    viewMode = "unified";
    btnUnified.classList.add("active");
    btnSplit.classList.remove("active");
    renderDiff();
  });

  // Initial render
  renderSidebar();
  renderDiff();
})();
