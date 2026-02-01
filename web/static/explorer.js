// Explorer — Tree-based API browser
// Discovers API resources via Link headers and presents them as a navigable tree.
'use strict';

// ---------------------------------------------------------------------------
// RFC 8288 Link header parser
// ---------------------------------------------------------------------------
function parseLinks(response) {
  const links = {};
  const raw = response.headers.get('Link');
  if (!raw) return links;
  for (const part of raw.split(',')) {
    const m = part.match(/<([^>]+)>;\s*rel="([^"]+)"/);
    if (!m) continue;
    const entry = { href: m[1] };
    const method = part.match(/;\s*method="([^"]+)"/);
    if (method) entry.method = method[1];
    const title = part.match(/;\s*title="([^"]+)"/);
    if (title) entry.title = title[1];
    const schema = part.match(/;\s*schema="([^"]+)"/);
    if (schema) entry.schema = schema[1];
    if (!links[m[2]]) links[m[2]] = [];
    links[m[2]].push(entry);
  }
  return links;
}

// ---------------------------------------------------------------------------
// OpenAPI helpers
// ---------------------------------------------------------------------------
let _specCache = null;

async function getSpec() {
  if (_specCache) return _specCache;
  const r = await fetch('/openapi.json');
  _specCache = await r.json();
  return _specCache;
}

function resolveRef(spec, ref) {
  if (!ref || !ref.startsWith('#/')) return null;
  let obj = spec;
  for (const p of ref.substring(2).split('/')) {
    obj = obj?.[p];
    if (!obj) return null;
  }
  return obj;
}

function findTemplatePath(spec, actual) {
  for (const p of Object.keys(spec.paths || {})) {
    if (!p.includes('{')) continue;
    const re = new RegExp('^' + p.replace(/\{[^}]+\}/g, '[^/]+') + '$');
    if (re.test(actual)) return p;
  }
  return null;
}

function operationSchema(spec, path, method) {
  const op = spec.paths?.[path]?.[method];
  const ct = op?.requestBody?.content?.['application/json'];
  if (!ct?.schema) return null;
  return ct.schema.$ref ? resolveRef(spec, ct.schema.$ref) : ct.schema;
}

async function resolveSchema(href, method, schemaUrl) {
  if (schemaUrl) {
    try {
      const r = await fetch(schemaUrl, { headers: { Accept: 'application/json' } });
      if (r.ok) return await r.json();
    } catch { /* fall through */ }
  }
  const spec = await getSpec();
  const tpl = findTemplatePath(spec, href) || href;
  return operationSchema(spec, tpl, method.toLowerCase());
}

// ---------------------------------------------------------------------------
// DOM helpers
// ---------------------------------------------------------------------------
function el(tag, attrs, children) {
  const e = document.createElement(tag);
  if (attrs) {
    for (const [k, v] of Object.entries(attrs)) {
      if (k === 'class') e.className = v;
      else if (k === 'disabled') e.disabled = !!v;
      else if (k === 'type') e.type = v;
      else if (k === 'href') e.href = v;
      else if (k === 'target') e.target = v;
      else e.setAttribute(k, v);
    }
  }
  if (typeof children === 'string') e.textContent = children;
  else if (Array.isArray(children)) for (const c of children) if (c) e.appendChild(c);
  return e;
}

// ---------------------------------------------------------------------------
// Schema-driven form helpers
// ---------------------------------------------------------------------------
function buildFormFields(schema, values, spec) {
  if (!schema?.properties) return [];
  const required = new Set(schema.required || []);
  return Object.entries(schema.properties)
    .filter(([name]) => name !== '$schema')
    .map(([name, prop]) => {
      if (prop.$ref && spec) prop = resolveRef(spec, prop.$ref) || prop;
      const val = values?.[name] ?? prop.default ?? '';
      const req = required.has(name);
      const group = el('div', { class: 'form-group' });
      const label = el('label');
      label.textContent = name;
      if (req) label.appendChild(el('span', { class: 'req' }, ' *'));
      group.appendChild(label);

      let input;
      if (prop.enum) {
        input = el('select', { name });
        for (const opt of prop.enum) {
          const o = el('option', { value: opt }, opt);
          if (opt === val) o.selected = true;
          input.appendChild(o);
        }
      } else if (prop.type === 'boolean') {
        input = el('input', { type: 'checkbox', name });
        if (val) input.checked = true;
      } else if (prop.type === 'number' || prop.type === 'integer') {
        const a = { type: 'number', name, value: String(val) };
        if (prop.minimum != null) a.min = String(prop.minimum);
        if (prop.maximum != null) a.max = String(prop.maximum);
        a.step = prop.type === 'integer' ? '1' : 'any';
        if (req) a.required = '';
        input = el('input', a);
      } else {
        const isColor = (typeof val === 'string' && /^#[0-9a-fA-F]{6}$/.test(val)) || /color|fill|stroke/i.test(name);
        const a = { type: isColor ? 'color' : 'text', name, value: String(val) };
        if (req) a.required = '';
        input = el('input', a);
      }
      if (input) group.appendChild(input);
      if (prop.description) group.appendChild(el('div', { class: 'hint' }, prop.description));
      return group;
    });
}

function collectFormData(formEl, schema) {
  const data = {};
  if (!schema?.properties) return data;
  for (const [name, prop] of Object.entries(schema.properties)) {
    const input = formEl.querySelector(`[name="${name}"]`);
    if (!input) continue;
    if (prop.type === 'boolean') data[name] = input.checked;
    else if (prop.type === 'number') data[name] = parseFloat(input.value);
    else if (prop.type === 'integer') data[name] = parseInt(input.value, 10);
    else if (input.value !== '') data[name] = input.value;
  }
  return data;
}

function showFormModal(title, schema, values, onSubmit) {
  const overlay = el('div', { class: 'form-overlay' });
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
  const panel = el('div', { class: 'form-panel' });
  const header = el('div', { class: 'panel-header' });
  header.appendChild(el('h2', null, title));
  const closeBtn = el('button', { class: 'btn btn-sm' }, '\u2715');
  closeBtn.addEventListener('click', () => overlay.remove());
  header.appendChild(closeBtn);
  panel.appendChild(header);

  const body = el('div', { class: 'panel-body' });
  const form = el('form');
  for (const field of buildFormFields(schema, values, _specCache)) form.appendChild(field);
  const actions = el('div', { class: 'form-actions' });
  const cancelBtn = el('button', { type: 'button', class: 'btn btn-sm' }, 'Cancel');
  cancelBtn.addEventListener('click', () => overlay.remove());
  actions.appendChild(cancelBtn);
  actions.appendChild(el('button', { type: 'submit', class: 'btn btn-primary btn-sm' }, 'Save'));
  form.appendChild(actions);
  body.appendChild(form);
  const errEl = el('div', { class: 'error' });
  errEl.style.display = 'none';
  body.appendChild(errEl);
  panel.appendChild(body);
  overlay.appendChild(panel);
  document.body.appendChild(overlay);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    try {
      errEl.style.display = 'none';
      await onSubmit(collectFormData(form, schema));
      overlay.remove();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.style.display = 'block';
    }
  });
}

// ---------------------------------------------------------------------------
// Tree node types
// ---------------------------------------------------------------------------
const COLLECTIONS = [
  { label: 'Layers',  rel: 'layers',  href: '/api/v1/layers',  icon: '\u25A0' },
  { label: 'Sources', rel: 'sources', href: '/api/v1/sources', icon: '\u25C6' },
  { label: 'Tiles',   rel: 'tiles',   href: '/api/v1/tiles',   icon: '\u25B2' },
  { label: 'Tables',  rel: 'tables',  href: '/api/v1/tables',  icon: '\u25CF' },
];

const SINGLETONS = [
  { label: 'Health', href: '/health',      icon: '\u2764' },
  { label: 'Info',   href: '/api/v1/info', icon: '\u24D8' },
];

// ---------------------------------------------------------------------------
// Explorer application
// ---------------------------------------------------------------------------
const Explorer = {
  state: { path: null, data: null, links: {} },
  treeState: {},  // href -> { expanded, items, count }
  $tree: null,
  $detail: null,

  init() {
    this.$tree = document.getElementById('tree');
    this.$detail = document.getElementById('detail');
    this.renderTree();
    this.navigate('/health');
  },

  // --- Tree sidebar ---
  renderTree() {
    this.$tree.innerHTML = '';

    // Collections
    for (const col of COLLECTIONS) {
      const ts = this.treeState[col.href] || {};
      const node = el('div', { class: 'tree-node' });

      const row = el('div', { class: 'tree-row' + (this.state.path === col.href ? ' active' : '') });
      const toggle = el('span', { class: 'tree-toggle' }, ts.expanded ? '\u25BC' : '\u25B6');
      toggle.addEventListener('click', (e) => { e.stopPropagation(); this.toggleCollection(col); });
      row.appendChild(toggle);
      row.appendChild(el('span', { class: 'tree-icon' }, col.icon));
      const label = el('span', { class: 'tree-label' }, col.label);
      row.appendChild(label);
      if (ts.count != null) row.appendChild(el('span', { class: 'tree-badge' }, String(ts.count)));
      row.addEventListener('click', () => this.navigate(col.href));
      node.appendChild(row);

      // Expanded children
      if (ts.expanded && ts.items) {
        const children = el('div', { class: 'tree-children' });
        for (const item of ts.items) {
          const itemRow = el('div', {
            class: 'tree-row tree-item' + (this.state.path === item.href ? ' active' : ''),
          });
          itemRow.appendChild(el('span', { class: 'tree-icon' }, '\u2022'));
          itemRow.appendChild(el('span', { class: 'tree-label' }, item.label));
          itemRow.addEventListener('click', () => this.navigate(item.href));
          children.appendChild(itemRow);
        }
        if (ts.items.length === 0) {
          children.appendChild(el('div', { class: 'tree-row tree-empty' }, 'No items'));
        }
        node.appendChild(children);
      }

      this.$tree.appendChild(node);
    }

    // Divider
    this.$tree.appendChild(el('div', { class: 'tree-divider' }));

    // Singletons
    for (const s of SINGLETONS) {
      const row = el('div', { class: 'tree-row' + (this.state.path === s.href ? ' active' : '') });
      row.appendChild(el('span', { class: 'tree-icon' }, s.icon));
      row.appendChild(el('span', { class: 'tree-label' }, s.label));
      row.addEventListener('click', () => this.navigate(s.href));
      this.$tree.appendChild(row);
    }

    // External links
    this.$tree.appendChild(el('div', { class: 'tree-divider' }));
    const docsRow = el('a', { class: 'tree-row', href: '/docs', target: '_blank' });
    docsRow.appendChild(el('span', { class: 'tree-icon' }, '\u2197' ));
    docsRow.appendChild(el('span', { class: 'tree-label' }, 'API Docs'));
    this.$tree.appendChild(docsRow);
  },

  async toggleCollection(col) {
    const ts = this.treeState[col.href] || {};
    if (ts.expanded) {
      ts.expanded = false;
      this.treeState[col.href] = ts;
      this.renderTree();
      return;
    }
    // Fetch items
    ts.expanded = true;
    ts.items = [];
    this.treeState[col.href] = ts;
    this.renderTree();

    try {
      const resp = await fetch(col.href, { headers: { Accept: 'application/json' } });
      if (!resp.ok) return;
      const data = await resp.json();
      const items = Array.isArray(data?.data) ? data.data : (Array.isArray(data) ? data : []);
      ts.count = data?.total ?? items.length;
      ts.items = items.map(item => {
        const id = item.id || item.name || item.filename || JSON.stringify(item).substring(0, 30);
        return { label: String(id), href: col.href + '/' + encodeURIComponent(id), raw: item };
      });
      this.renderTree();
    } catch { /* ignore */ }
  },

  // --- Navigation ---
  async navigate(href) {
    this.state.path = href;
    this.renderTree(); // update active highlight
    this.$detail.innerHTML = '';
    this.$detail.appendChild(el('div', { class: 'loading' }, 'Loading...'));

    try {
      const resp = await fetch(href, { headers: { Accept: 'application/json' } });
      const links = parseLinks(resp);
      this.state.links = links;

      if (!resp.ok) {
        this.$detail.innerHTML = '';
        this.$detail.appendChild(el('div', { class: 'error' }, 'HTTP ' + resp.status + ': ' + resp.statusText));
        return;
      }

      const data = await resp.json();
      this.state.data = data;
      const displayData = Array.isArray(data?.data) ? data.data : data;
      const isArray = Array.isArray(displayData);

      this.$detail.innerHTML = '';

      // Welcome panel on health page
      if (href === '/health') {
        const welcome = el('div', { class: 'panel welcome-panel' });
        const wBody = el('div', { class: 'panel-body' });
        wBody.appendChild(el('h2', { class: 'welcome-title' }, 'Welcome to the API Explorer'));
        wBody.appendChild(el('p', { class: 'welcome-text' },
          'Browse your geospatial data using the tree on the left. ' +
          'Click a collection (Layers, Sources, Tiles, Tables) to expand it and see items inside. ' +
          'Click any item to view its details, edit it, or run actions like Publish or Delete.'));
        const tips = el('div', { class: 'welcome-tips' });
        tips.appendChild(el('div', null, '\u25B6  Click a collection name to expand it'));
        tips.appendChild(el('div', null, '\u2022  Click an item to see its properties'));
        tips.appendChild(el('div', null, '\u270E  Use action buttons to create, edit, or delete'));
        tips.appendChild(el('div', null, '\u25B8  Raw JSON and Link Headers are available at the bottom of each page'));
        wBody.appendChild(tips);
        welcome.appendChild(wBody);
        this.$detail.appendChild(welcome);
      }

      // Breadcrumb
      this.$detail.appendChild(this.buildBreadcrumb(href));

      // Actions
      const actionsBar = this.buildActions(links);
      if (actionsBar.childNodes.length > 0) {
        const actionsPanel = el('div', { class: 'panel actions-panel' });
        actionsPanel.appendChild(actionsBar);
        this.$detail.appendChild(actionsPanel);
      }

      // Data panel
      const panel = el('div', { class: 'panel' });
      const header = el('div', { class: 'panel-header' });
      const title = this.friendlyTitle(href);
      const count = data?.total != null ? data.total : (isArray ? displayData.length : null);
      header.appendChild(el('h2', null, title + (count != null ? ' (' + count + ')' : '')));
      panel.appendChild(header);

      const body = el('div', { class: 'panel-body' });
      body.appendChild(isArray ? this.buildTable(displayData) : this.buildDetail(displayData));
      body.appendChild(this.buildPagination(links));
      panel.appendChild(body);
      this.$detail.appendChild(panel);

      // Related resources (sub-collections from links)
      const related = this.buildRelated(links);
      if (related) this.$detail.appendChild(related);

      // Collapsible: Raw JSON
      this.$detail.appendChild(this.buildCollapsible('Raw JSON', el('pre', { class: 'raw-json' }, JSON.stringify(data, null, 2))));

      // Collapsible: Link Headers
      const linkEntries = Object.entries(links).flatMap(([rel, items]) =>
        items.map(item => ({ rel, ...item }))
      );
      if (linkEntries.length > 0) {
        const linkList = el('div', { class: 'link-list' });
        for (const e of linkEntries) {
          const row = el('div', { class: 'link-row' });
          row.appendChild(el('span', { class: 'link-rel' }, e.rel));
          if (e.method) row.appendChild(el('span', { class: 'link-method' }, e.method));
          const a = el('a', { class: 'link-href' });
          a.textContent = e.title || e.href;
          a.addEventListener('click', () => this.navigate(e.href));
          row.appendChild(a);
          linkList.appendChild(row);
        }
        this.$detail.appendChild(this.buildCollapsible('Link Headers (' + linkEntries.length + ')', linkList));
      }

    } catch (err) {
      this.$detail.innerHTML = '';
      this.$detail.appendChild(el('div', { class: 'error' }, 'Error: ' + err.message));
    }
  },

  friendlyTitle(href) {
    const segs = href.split('?')[0].split('/').filter(Boolean);
    return segs[segs.length - 1] || 'API';
  },

  // --- Breadcrumb ---
  buildBreadcrumb(href) {
    const nav = el('div', { class: 'breadcrumb' });
    const homeLink = el('a');
    homeLink.textContent = 'API';
    homeLink.addEventListener('click', () => this.navigate('/health'));
    nav.appendChild(homeLink);

    const parts = href.split('?')[0].split('/').filter(Boolean);
    let built = '';
    for (const part of parts) {
      built += '/' + part;
      nav.appendChild(el('span', { class: 'sep' }, ' / '));
      const a = el('a');
      a.textContent = part;
      const target = built;
      a.addEventListener('click', () => this.navigate(target));
      nav.appendChild(a);
    }
    return nav;
  },

  // --- Actions ---
  buildActions(links) {
    const bar = el('div', { class: 'actions' });

    if (links['create-form']) {
      const btn = el('button', { class: 'btn btn-primary' }, '+ New');
      btn.addEventListener('click', () => this.showCreateForm());
      bar.appendChild(btn);
    }
    if (links['edit'] || links['edit-form']) {
      const btn = el('button', { class: 'btn btn-primary' }, 'Edit');
      btn.addEventListener('click', () => this.showEditForm());
      bar.appendChild(btn);
    }

    const skip = new Set(['self', 'up', 'collection', 'create-form', 'edit-form', 'edit',
      'item', 'first', 'prev', 'next', 'last', 'search', 'describedby', 'service-desc', 'service-doc']);
    for (const [rel, items] of Object.entries(links)) {
      if (skip.has(rel)) continue;
      for (const item of items) {
        if (!item.method) continue;
        const label = item.title || rel;
        const cls = item.method === 'DELETE' ? 'btn btn-danger' : 'btn btn-action';
        const btn = el('button', { class: cls }, label);
        btn.addEventListener('click', () => this.executeAction(item.href, item.method, label, item.schema));
        bar.appendChild(btn);
      }
    }

    return bar;
  },

  // --- Table ---
  buildTable(data) {
    if (!Array.isArray(data) || data.length === 0) return el('p', { class: 'empty' }, 'No items.');
    const keys = Object.keys(data[0]);
    const table = el('table');
    const thead = el('thead');
    const headRow = el('tr');
    for (const k of keys) headRow.appendChild(el('th', null, k));
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = el('tbody');
    for (const row of data) {
      const tr = el('tr');
      const id = row.id || row.name || row.filename;
      const basePath = this.state.path.split('?')[0];
      const itemHref = id ? basePath + '/' + encodeURIComponent(id) : null;
      if (itemHref) tr.classList.add('clickable');

      for (const k of keys) {
        const v = row[k];
        const td = el('td');
        if (k === 'id' || k === 'name' || k === 'filename') {
          if (itemHref) {
            const a = el('a');
            a.textContent = String(v ?? '');
            a.addEventListener('click', (e) => { e.stopPropagation(); this.navigate(itemHref); });
            td.appendChild(a);
          } else {
            td.textContent = String(v ?? '');
          }
        } else if (typeof v === 'object' && v !== null) {
          td.textContent = JSON.stringify(v);
        } else {
          td.textContent = String(v ?? '');
        }
        tr.appendChild(td);
      }
      if (itemHref) tr.addEventListener('click', () => this.navigate(itemHref));
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    return table;
  },

  // --- Detail ---
  buildDetail(data) {
    if (typeof data !== 'object' || data === null) return el('pre', null, JSON.stringify(data, null, 2));
    const grid = el('div', { class: 'detail-grid' });
    for (const [k, v] of Object.entries(data)) {
      grid.appendChild(el('div', { class: 'detail-key' }, k));
      const val = el('div', { class: 'detail-value' });
      if (k.toLowerCase().includes('color') && typeof v === 'string' && /^#[0-9a-fA-F]{3,8}$/.test(v)) {
        const swatch = el('span', { class: 'color-swatch' });
        swatch.style.backgroundColor = v;
        val.appendChild(swatch);
        val.appendChild(document.createTextNode(' ' + v));
      } else if (typeof v === 'boolean') {
        val.textContent = v ? 'Yes' : 'No';
      } else if (typeof v === 'object') {
        val.textContent = JSON.stringify(v);
      } else {
        val.textContent = String(v);
      }
      grid.appendChild(val);
    }
    return grid;
  },

  // --- Related resources ---
  buildRelated(links) {
    const related = [];
    const itemCount = links.item ? links.item.length : 0;
    if (itemCount > 0) related.push({ label: 'Items (' + itemCount + ')', href: null });

    // Sub-resources from links (e.g. styles)
    const skip = new Set(['self', 'up', 'collection', 'create-form', 'edit-form', 'edit',
      'item', 'first', 'prev', 'next', 'last', 'search', 'describedby', 'service-desc', 'service-doc']);
    for (const [rel, items] of Object.entries(links)) {
      if (skip.has(rel)) continue;
      for (const item of items) {
        if (item.method) continue; // actions, not sub-resources
        related.push({ label: item.title || rel, href: item.href });
      }
    }

    if (related.length === 0) return null;

    const panel = el('div', { class: 'panel' });
    const header = el('div', { class: 'panel-header' });
    header.appendChild(el('h2', null, 'Related'));
    panel.appendChild(header);
    const body = el('div', { class: 'panel-body' });
    for (const r of related) {
      if (r.href) {
        const a = el('a', { class: 'related-link' });
        a.textContent = '\u2192 ' + r.label;
        a.addEventListener('click', () => this.navigate(r.href));
        body.appendChild(a);
      } else {
        body.appendChild(el('div', { class: 'related-info' }, r.label));
      }
    }
    panel.appendChild(body);
    return panel;
  },

  // --- Pagination ---
  buildPagination(links) {
    const container = el('div', { class: 'pagination' });
    const has = ['first', 'prev', 'next', 'last'].some(r => links[r]);
    if (!has) return container;
    for (const [rel, label] of [['first', '\u00AB First'], ['prev', '\u2039 Prev'], ['next', 'Next \u203A'], ['last', 'Last \u00BB']]) {
      if (links[rel]) {
        const btn = el('button', { class: 'btn btn-sm' });
        btn.textContent = label;
        btn.addEventListener('click', () => this.navigate(links[rel][0].href));
        container.appendChild(btn);
      } else {
        container.appendChild(el('button', { class: 'btn btn-sm', disabled: true }, label));
      }
    }
    return container;
  },

  // --- Collapsible section ---
  buildCollapsible(title, content) {
    const wrapper = el('div', { class: 'collapsible' });
    const toggle = el('div', { class: 'collapsible-toggle' });
    toggle.appendChild(el('span', { class: 'collapsible-arrow' }, '\u25B6'));
    toggle.appendChild(el('span', null, title));
    const body = el('div', { class: 'collapsible-body' });
    body.style.display = 'none';
    body.appendChild(content);
    toggle.addEventListener('click', () => {
      const open = body.style.display !== 'none';
      body.style.display = open ? 'none' : 'block';
      toggle.querySelector('.collapsible-arrow').textContent = open ? '\u25B6' : '\u25BC';
    });
    wrapper.appendChild(toggle);
    wrapper.appendChild(body);
    return wrapper;
  },

  // --- Form actions ---
  async showCreateForm() {
    const basePath = this.state.path.split('?')[0];
    const schema = await resolveSchema(basePath, 'POST', null);
    if (!schema) { alert('No POST schema found for ' + basePath); return; }
    showFormModal('Create New', schema, {}, async (formData) => {
      const resp = await fetch(basePath, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify(formData),
      });
      if (!resp.ok) {
        const err = await resp.json().catch(() => ({}));
        throw new Error(err.detail || err.message || 'HTTP ' + resp.status);
      }
      this.navigate(this.state.path);
    });
  },

  async showEditForm() {
    const basePath = this.state.path.split('?')[0];
    const schema = await resolveSchema(basePath, 'PUT', null) || await resolveSchema(basePath, 'PATCH', null);
    if (!schema) { alert('No PUT/PATCH schema found for ' + basePath); return; }
    showFormModal('Edit', schema, this.state.data || {}, async (formData) => {
      const resp = await fetch(basePath, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify(formData),
      });
      if (!resp.ok) {
        const err = await resp.json().catch(() => ({}));
        throw new Error(err.detail || err.message || 'HTTP ' + resp.status);
      }
      this.navigate(this.state.path);
    });
  },

  async executeAction(href, method, label, schemaUrl) {
    const schema = await resolveSchema(href, method, schemaUrl);
    if (schema?.properties && Object.keys(schema.properties).length > 0) {
      showFormModal(label, schema, {}, async (formData) => {
        const resp = await fetch(href, {
          method,
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body: JSON.stringify(formData),
        });
        if (!resp.ok) {
          const err = await resp.json().catch(() => ({}));
          throw new Error(err.detail || err.message || 'HTTP ' + resp.status);
        }
        this.navigate(this.state.path);
      });
    } else {
      if (!confirm('Execute "' + label + '" (' + method + ' ' + href + ')?')) return;
      const resp = await fetch(href, {
        method,
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      });
      if (!resp.ok) {
        const err = await resp.json().catch(() => ({}));
        alert('Error: ' + (err.detail || err.message || resp.statusText));
        return;
      }
      this.navigate(this.state.path);
    }
  },
};

document.addEventListener('DOMContentLoaded', () => Explorer.init());
