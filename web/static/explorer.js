// Explorer — HATEOAS API browser
// Discovers and navigates RFC 8288 Link headers.
'use strict';

// ---------------------------------------------------------------------------
// Rel configuration — single source of truth for classification & rendering
// ---------------------------------------------------------------------------
const REL_CONFIG = {
  self:           { category: 'meta' },
  up:             { category: 'nav',    label: '\u2191 Up' },
  collection:     { category: 'nav',    label: 'Back to List' },
  'create-form':  { category: 'action', label: '+ Add New',  btnClass: 'btn-primary' },
  'edit-form':    { category: 'action', label: 'Edit',       btnClass: 'btn-primary' },
  edit:           { category: 'action', label: 'Edit',       btnClass: 'btn-primary' },
  search:         { category: 'meta' },
  describedby:    { category: 'meta' },
  'service-desc': { category: 'meta' },
  'service-doc':  { category: 'meta' },
  first:          { category: 'page' },
  prev:           { category: 'page' },
  next:           { category: 'page' },
  last:           { category: 'page' },
};

function relCategory(rel) {
  return REL_CONFIG[rel]?.category ?? '';
}

function relCSSClass(rel) {
  const cat = relCategory(rel);
  if (cat === 'action') return 'action';
  if (cat === 'meta')   return 'meta';
  return '';
}

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

/** Unified schema resolution: Link header schema URL → OpenAPI fallback. */
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
// DOM helpers — safe element creation
// ---------------------------------------------------------------------------

/** Create an element. attrs is a plain object, children is string | Node[]. */
function el(tag, attrs, children) {
  const e = document.createElement(tag);
  if (attrs) {
    for (const [k, v] of Object.entries(attrs)) {
      if (k === 'class') { e.className = v; }
      else if (k.startsWith('data-')) { e.setAttribute(k, v); }
      else if (k === 'disabled') { e.disabled = !!v; }
      else if (k === 'type') { e.type = v; }
      else if (k === 'href') { e.href = v; }
      else if (k === 'target') { e.target = v; }
      else { e.setAttribute(k, v); }
    }
  }
  if (typeof children === 'string') {
    e.textContent = children;
  } else if (Array.isArray(children)) {
    for (const c of children) if (c) e.appendChild(c);
  }
  return e;
}

function text(s) { return document.createTextNode(s); }

// ---------------------------------------------------------------------------
// Schema-driven form generation
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
      const doc = prop.description || '';

      const group = el('div', { class: 'form-group' });
      const label = el('label');
      label.textContent = name;
      if (req) {
        const star = el('span', { class: 'req' }, ' *');
        label.appendChild(star);
      }
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
        // Wrap checkbox in label
        const wrap = el('label');
        wrap.appendChild(input);
        wrap.appendChild(text(' ' + name));
        group.innerHTML = '';
        group.appendChild(wrap);
      } else if (prop.type === 'number' || prop.type === 'integer') {
        const attrs = { type: 'number', name, value: String(val) };
        if (prop.minimum != null) attrs.min = String(prop.minimum);
        if (prop.maximum != null) attrs.max = String(prop.maximum);
        attrs.step = prop.type === 'integer' ? '1' : 'any';
        if (req) attrs.required = '';
        input = el('input', attrs);
      } else {
        const isColor = (typeof val === 'string' && /^#[0-9a-fA-F]{6}$/.test(val)) ||
                         /color|fill|stroke/i.test(name);
        const attrs = { type: isColor ? 'color' : 'text', name, value: String(val) };
        if (prop.minLength) attrs.minlength = String(prop.minLength);
        if (prop.maxLength) attrs.maxlength = String(prop.maxLength);
        if (req) attrs.required = '';
        input = el('input', attrs);
      }

      if (input) group.appendChild(input);
      if (doc) group.appendChild(el('div', { class: 'hint' }, doc));
      return group;
    });
}

function collectFormData(formEl, schema) {
  const data = {};
  if (!schema?.properties) return data;
  for (const [name, prop] of Object.entries(schema.properties)) {
    const input = formEl.querySelector(`[name="${name}"]`);
    if (!input) continue;
    if (prop.type === 'boolean')      data[name] = input.checked;
    else if (prop.type === 'number')  data[name] = parseFloat(input.value);
    else if (prop.type === 'integer') data[name] = parseInt(input.value, 10);
    else if (input.value !== '')      data[name] = input.value;
  }
  return data;
}

// ---------------------------------------------------------------------------
// Form modal
// ---------------------------------------------------------------------------
function showFormModal(title, schema, values, onSubmit) {
  const overlay = el('div', { class: 'form-overlay' });
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });

  const panel = el('div', { class: 'form-panel' });

  // Header
  const header = el('div', { class: 'panel-header' });
  header.appendChild(el('h2', null, title));
  const closeBtn = el('button', { class: 'btn btn-sm' }, '\u2715');
  closeBtn.addEventListener('click', () => overlay.remove());
  header.appendChild(closeBtn);
  panel.appendChild(header);

  // Body
  const body = el('div', { class: 'panel-body' });
  const form = el('form');
  for (const field of buildFormFields(schema, values, _specCache)) {
    form.appendChild(field);
  }
  const formActions = el('div', { class: 'form-actions' });
  const cancelBtn = el('button', { type: 'button', class: 'btn btn-sm' }, 'Cancel');
  cancelBtn.addEventListener('click', () => overlay.remove());
  formActions.appendChild(cancelBtn);
  formActions.appendChild(el('button', { type: 'submit', class: 'btn btn-primary btn-sm' }, 'Save'));
  form.appendChild(formActions);
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
// Explorer application
// ---------------------------------------------------------------------------
const Explorer = {
  state: { path: null, data: null, links: {} },
  $content: null,
  $nav: null,

  init() {
    this.$content = document.getElementById('content');
    this.$nav = document.getElementById('nav-collections');

    // Event delegation for navigation links
    document.addEventListener('click', (e) => {
      const a = e.target.closest('[data-navigate]');
      if (a) { e.preventDefault(); this.navigate(a.dataset.navigate); }
    });

    this.discover();
  },

  async discover() {
    try {
      const resp = await fetch('/health', { headers: { Accept: 'application/json' } });
      const links = parseLinks(resp);
      this.renderNav(links);
      this.navigate('/health');
    } catch (err) {
      this.$content.innerHTML = '';
      this.$content.appendChild(
        el('div', { class: 'error' }, 'Failed to discover API: ' + err.message + '. Is the server running?')
      );
    }
  },

  renderNav(links) {
    this.$nav.innerHTML = '';
    const label = el('div', { class: 'nav-label' }, 'Collections');
    this.$nav.appendChild(label);

    const entries = Object.entries(links)
      .filter(([rel]) => relCategory(rel) !== 'meta' && relCategory(rel) !== 'page');

    if (entries.length === 0) {
      this.$nav.appendChild(el('div', { class: 'nav-label' }, 'No links discovered'));
      return;
    }

    for (const [rel, items] of entries) {
      for (const item of items) {
        const a = el('a', { 'data-navigate': item.href, 'data-nav-href': item.href });
        a.appendChild(text(rel + ' '));
        a.appendChild(el('span', { class: 'rel-badge' }, rel));
        this.$nav.appendChild(a);
      }
    }
  },

  async navigate(href) {
    this.$content.innerHTML = '';
    this.$content.appendChild(el('div', { class: 'loading' }, 'Loading...'));
    this.state.path = href;

    try {
      const resp = await fetch(href, { headers: { Accept: 'application/json' } });
      const links = parseLinks(resp);
      this.state.links = links;

      if (!resp.ok) {
        this.$content.innerHTML = '';
        this.$content.appendChild(el('div', { class: 'error' }, 'HTTP ' + resp.status + ': ' + resp.statusText));
        return;
      }

      const data = await resp.json();
      this.state.data = data;

      // Detect paginated envelope
      const displayData = Array.isArray(data?.data) ? data.data : data;
      const isArray = Array.isArray(displayData);
      const pathName = href.split('?')[0].split('/').filter(Boolean).pop() || 'API';
      const countLabel = data?.total != null ? '(' + data.total + ' total)'
                       : isArray ? '(' + displayData.length + ')' : '';

      // Highlight active nav
      document.querySelectorAll('#sidebar a').forEach(a => a.classList.remove('active'));
      const basePath = href.split('?')[0];
      const active = document.querySelector('#sidebar a[data-nav-href="' + basePath + '"]');
      if (active) active.classList.add('active');

      // Build content
      this.$content.innerHTML = '';

      // Discovery banner — explains what just happened
      const totalLinks = Object.values(links).reduce((n, arr) => n + arr.length, 0);
      const relCount = Object.keys(links).length;
      this.$content.appendChild(this.buildDiscoveryBanner(href, totalLinks, relCount));

      this.$content.appendChild(this.buildBreadcrumb(href));

      // Mesh graph — shows the hypermedia link topology
      this.$content.appendChild(this.buildMeshPanel(links, href));

      // Main data panel
      const panel = el('div', { class: 'panel' });
      const header = el('div', { class: 'panel-header' });
      header.appendChild(el('h2', null, pathName + ' ' + countLabel));
      header.appendChild(this.buildActions(links));
      panel.appendChild(header);

      const body = el('div', { class: 'panel-body' });
      body.appendChild(isArray ? this.buildTable(displayData) : this.buildDetail(displayData));
      body.appendChild(this.buildPagination(links));
      panel.appendChild(body);
      this.$content.appendChild(panel);

      // Links panel
      this.$content.appendChild(this.buildLinksPanel(links));

      // Raw JSON panel
      this.$content.appendChild(this.buildRawPanel(data));

    } catch (err) {
      this.$content.innerHTML = '';
      this.$content.appendChild(el('div', { class: 'error' }, 'Error: ' + err.message));
    }
  },

  // --- Mesh graph ---
  buildMeshPanel(links, currentHref) {
    const panel = el('div', { class: 'panel' });
    const header = el('div', { class: 'panel-header' });
    header.appendChild(el('h2', null, 'Hypermedia Mesh'));
    panel.appendChild(header);

    const mesh = el('div', { class: 'mesh' });

    // Classify links into spatial zones
    const upNodes = [];    // up, collection
    const downNodes = [];  // item
    const leftNodes = [];  // sibling collections (first half)
    const rightNodes = []; // sibling collections (second half)
    const actions = [];    // method-bearing rels (badges on current node)

    for (const [rel, items] of Object.entries(links)) {
      const cat = relCategory(rel);
      if (cat === 'page' || cat === 'meta') continue;
      if (rel === 'self') continue;

      for (const item of items) {
        if (rel === 'up' || rel === 'collection') {
          upNodes.push({ rel, href: item.href, label: this._nodeLabel(item.href) });
        } else if (rel === 'item') {
          downNodes.push({ rel, href: item.href, label: this._nodeLabel(item.href) });
        } else if (item.method) {
          actions.push({ rel, href: item.href, method: item.method, title: item.title, schema: item.schema });
        } else {
          // Sibling — navigable, no method
          const arr = leftNodes.length <= rightNodes.length ? leftNodes : rightNodes;
          arr.push({ rel, href: item.href, label: this._nodeLabel(item.href) });
        }
      }
    }

    // Up zone
    const upZone = el('div', { class: 'mesh-up' });
    for (const n of upNodes) {
      upZone.appendChild(this._meshNode(n.href, n.label, n.rel, false));
    }
    if (upNodes.length > 0) {
      upZone.appendChild(el('div', { class: 'mesh-arrow' }, '\u2193'));
    }
    mesh.appendChild(upZone);

    // Left zone
    const leftZone = el('div', { class: 'mesh-left' });
    for (const n of leftNodes) {
      leftZone.appendChild(this._meshNode(n.href, n.label, n.rel, false));
    }
    if (leftNodes.length > 0) {
      leftZone.appendChild(el('div', { class: 'mesh-arrow' }, '\u2192'));
    }
    mesh.appendChild(leftZone);

    // Center: current node
    const centerZone = el('div', { class: 'mesh-center' });
    const currentNode = el('div', { class: 'mesh-node current' });
    currentNode.appendChild(el('div', { class: 'mesh-node-label' }, this._nodeLabel(currentHref)));
    currentNode.appendChild(el('div', { class: 'mesh-node-path' }, currentHref.split('?')[0]));

    // Action badges on current node
    if (actions.length > 0 || links['create-form'] || links['edit'] || links['edit-form']) {
      const badgeBar = el('div', { class: 'mesh-node-actions' });
      if (links['create-form']) {
        const b = el('button', { class: 'mesh-action-badge create' }, '+ Add');
        b.addEventListener('click', (e) => { e.stopPropagation(); this.showCreateForm(); });
        badgeBar.appendChild(b);
      }
      if (links['edit'] || links['edit-form']) {
        const b = el('button', { class: 'mesh-action-badge edit' }, 'Edit');
        b.addEventListener('click', (e) => { e.stopPropagation(); this.showEditForm(); });
        badgeBar.appendChild(b);
      }
      for (const a of actions) {
        const label = a.title || a.rel;
        const cls = a.method === 'DELETE' ? 'mesh-action-badge delete' : 'mesh-action-badge default';
        const b = el('button', { class: cls }, label);
        b.addEventListener('click', (e) => {
          e.stopPropagation();
          this.executeAction(a.href, a.method, label, a.schema);
        });
        badgeBar.appendChild(b);
      }
      currentNode.appendChild(badgeBar);
    }
    centerZone.appendChild(currentNode);
    mesh.appendChild(centerZone);

    // Right zone
    const rightZone = el('div', { class: 'mesh-right' });
    if (rightNodes.length > 0) {
      rightZone.appendChild(el('div', { class: 'mesh-arrow' }, '\u2190'));
    }
    for (const n of rightNodes) {
      rightZone.appendChild(this._meshNode(n.href, n.label, n.rel, false));
    }
    mesh.appendChild(rightZone);

    // Down zone
    const downZone = el('div', { class: 'mesh-down' });
    if (downNodes.length > 0) {
      downZone.appendChild(el('div', { class: 'mesh-arrow' }, '\u2193'));
    }
    for (const n of downNodes) {
      downZone.appendChild(this._meshNode(n.href, n.label, n.rel, false));
    }
    mesh.appendChild(downZone);

    panel.appendChild(mesh);

    // Contextual hint
    const hint = el('div', { class: 'mesh-hint' });
    hint.appendChild(text('Click any node to navigate. The mesh redraws from the new resource\u2019s Link headers. '));
    hint.appendChild(el('strong', null, '\u2191'));
    hint.appendChild(text(' parent \u00B7 '));
    hint.appendChild(el('strong', null, '\u2190\u2192'));
    hint.appendChild(text(' siblings \u00B7 '));
    hint.appendChild(el('strong', null, '\u2193'));
    hint.appendChild(text(' children \u00B7 badges = actions'));
    panel.appendChild(hint);

    return panel;
  },

  _meshNode(href, label, rel, isCurrent) {
    const node = el('div', {
      class: 'mesh-node' + (isCurrent ? ' current' : ''),
      'data-navigate': href,
    });
    node.appendChild(el('div', { class: 'mesh-node-label' }, label));
    node.appendChild(el('div', { class: 'mesh-node-rel' }, rel));
    return node;
  },

  _nodeLabel(href) {
    const path = href.split('?')[0];
    const segs = path.split('/').filter(Boolean);
    return segs[segs.length - 1] || '/';
  },

  // --- Discovery banner ---
  buildDiscoveryBanner(href, totalLinks, relCount) {
    const banner = el('div', { class: 'discovery-banner' });
    banner.appendChild(el('span', { class: 'banner-icon' }, '\u{1F517}'));
    const msg = el('span');
    msg.appendChild(text('Fetched '));
    msg.appendChild(el('code', null, href));
    msg.appendChild(text(' \u2192 discovered '));
    msg.appendChild(el('strong', null, totalLinks + ' links'));
    msg.appendChild(text(' across ' + relCount + ' IANA rels. '));
    msg.appendChild(text('Every link below came from RFC 8288 Link headers \u2014 no hardcoded URLs.'));
    banner.appendChild(msg);
    return banner;
  },

  // --- Breadcrumb ---
  buildBreadcrumb(href) {
    const nav = el('div', { class: 'breadcrumb' });
    nav.appendChild(el('a', { 'data-navigate': '/health' }, 'API'));
    const parts = href.split('?')[0].split('/').filter(Boolean);
    let built = '';
    for (const part of parts) {
      built += '/' + part;
      nav.appendChild(el('span', null, ' \u203A '));
      nav.appendChild(el('a', { 'data-navigate': built }, part));
    }
    return nav;
  },

  // --- Actions toolbar ---
  buildActions(links) {
    const bar = el('div', { class: 'actions' });

    // Navigation rels
    for (const rel of ['up', 'collection']) {
      if (!links[rel]) continue;
      const cfg = REL_CONFIG[rel];
      const btn = el('button', { class: 'btn btn-sm', 'data-navigate': links[rel][0].href }, cfg.label);
      bar.appendChild(btn);
    }

    // Standard form rels
    if (links['create-form']) {
      const btn = el('button', { class: 'btn btn-primary btn-sm' }, '+ Add New');
      btn.addEventListener('click', () => this.showCreateForm());
      bar.appendChild(btn);
    }
    if (links['edit'] || links['edit-form']) {
      const btn = el('button', { class: 'btn btn-primary btn-sm' }, 'Edit');
      btn.addEventListener('click', () => this.showEditForm());
      bar.appendChild(btn);
    }

    // State-dependent action buttons (method-bearing links not handled above)
    const handled = new Set(['up', 'collection', 'create-form', 'edit-form', 'edit']);
    for (const [rel, items] of Object.entries(links)) {
      if (handled.has(rel) || relCategory(rel) === 'meta' || relCategory(rel) === 'page' || rel === 'self' || rel === 'item') continue;
      for (const item of items) {
        if (!item.method) continue;
        const label = item.title || rel;
        const cls = item.method === 'DELETE' ? 'btn btn-danger btn-sm' : 'btn btn-primary btn-sm';
        const btn = el('button', { class: cls }, label);
        btn.addEventListener('click', () => this.executeAction(item.href, item.method, label, item.schema));
        bar.appendChild(btn);
      }
    }

    return bar;
  },

  // --- Data table ---
  buildTable(data) {
    if (!Array.isArray(data) || data.length === 0) return el('p', { class: 'loading' }, 'No items.');
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
      for (const k of keys) {
        const v = row[k];
        const td = el('td');
        if (typeof v === 'object' && v !== null) {
          td.textContent = JSON.stringify(v);
        } else {
          td.textContent = String(v ?? '');
        }
        tr.appendChild(td);
      }
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    return table;
  },

  // --- Detail view ---
  buildDetail(data) {
    if (typeof data !== 'object' || data === null) {
      return el('pre', null, JSON.stringify(data, null, 2));
    }
    if (Array.isArray(data)) return this.buildTable(data);

    const grid = el('div', { class: 'detail-grid' });
    for (const [k, v] of Object.entries(data)) {
      grid.appendChild(el('div', { class: 'detail-key' }, k));
      const val = el('div', { class: 'detail-value' });
      val.textContent = typeof v === 'object' ? JSON.stringify(v) : String(v);
      grid.appendChild(val);
    }
    return grid;
  },

  // --- Pagination ---
  buildPagination(links) {
    const container = el('div', { class: 'pagination' });
    const has = ['first', 'prev', 'next', 'last'].some(r => links[r]);
    if (!has) return container;

    for (const [rel, label] of [['first', '\u00AB First'], ['prev', '\u2039 Prev'], ['next', 'Next \u203A'], ['last', 'Last \u00BB']]) {
      if (links[rel]) {
        container.appendChild(el('button', { class: 'btn btn-sm', 'data-navigate': links[rel][0].href }, label));
      } else {
        container.appendChild(el('button', { class: 'btn btn-sm', disabled: true }, label));
      }
    }
    return container;
  },

  // --- Links panel ---
  buildLinksPanel(links) {
    const entries = Object.entries(links).flatMap(([rel, items]) =>
      items.map(item => ({ rel, href: item.href, method: item.method, title: item.title }))
    );
    if (entries.length === 0) return document.createDocumentFragment();

    const panel = el('div', { class: 'panel' });
    const header = el('div', { class: 'panel-header' });
    header.appendChild(el('h2', null, 'Link Relations (RFC 8288)'));
    header.appendChild(el('span', { class: 'mesh-hint' }, entries.length + ' links from response headers'));
    panel.appendChild(header);

    const body = el('div', { class: 'panel-body' });
    const ul = el('ul', { class: 'link-list' });

    for (const e of entries) {
      const li = el('li');
      li.appendChild(el('span', { class: 'rel-tag ' + relCSSClass(e.rel) }, e.rel));
      if (e.method) li.appendChild(el('span', { class: 'rel-badge' }, e.method));
      const href = el('span', { class: 'link-href' });
      href.appendChild(el('a', { 'data-navigate': e.href }, e.title || e.href));
      li.appendChild(href);
      ul.appendChild(li);
    }

    body.appendChild(ul);
    panel.appendChild(body);
    return panel;
  },

  // --- Raw JSON panel ---
  buildRawPanel(data) {
    const panel = el('div', { class: 'panel' });
    const header = el('div', { class: 'panel-header' });
    header.appendChild(el('h2', null, 'Raw Response'));
    const toggleBtn = el('button', { class: 'btn btn-sm' }, 'Toggle JSON');
    toggleBtn.addEventListener('click', () => {
      rawDiv.classList.toggle('visible');
    });
    header.appendChild(toggleBtn);
    panel.appendChild(header);

    const rawDiv = el('div', { class: 'raw-json' });
    rawDiv.textContent = JSON.stringify(data, null, 2);
    panel.appendChild(rawDiv);
    return panel;
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
    const schema = await resolveSchema(basePath, 'PUT', null)
                || await resolveSchema(basePath, 'PATCH', null);
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

// Boot
document.addEventListener('DOMContentLoaded', () => Explorer.init());
