#!/usr/bin/env node
/**
 * i18n 缺键扫描:静态提取代码里 t('key') / t('ns:key') / i18next.t('ns:key')
 * 的字面量键,与 zh-CN / en-US 语言包 diff,缺键即裸 key 直出(页面显示 "action")。
 *
 * 键存在判定同时支持平铺点号键("success.true": "…")与嵌套结构
 * (i18next v26 ignoreJSONStructure 默认开,两种写法都有效)。
 *
 * 用法: node scripts/check-i18n-keys.mjs (或 pnpm check:i18n)
 * 退出码: 有缺键 = 1,可用于验收卡点。
 *
 * 已知局限(宁漏报勿误报):只提取单引号/双引号字面量键;模板字符串与变量键
 * 无法静态判定,不检查;跨行的 t(\n'key') 不识别。
 */
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(fileURLToPath(new URL('.', import.meta.url)), '..');
const srcDir = join(root, 'src');
const localeDirs = { 'zh-CN': join(root, 'src', 'locales', 'zh-CN'), 'en-US': join(root, 'src', 'locales', 'en-US') };

// ---------- 语言包装载(_core 与 _modules 同名合并,modules 覆盖 core,同 index.ts) ----------
function loadNamespaces(localeDir) {
  const map = {};
  for (const sub of ['_core', '_modules']) {
    const dir = join(localeDir, sub);
    let entries = [];
    try {
      entries = readdirSync(dir);
    } catch {
      continue;
    }
    for (const f of entries) {
      if (!f.endsWith('.json')) continue;
      const ns = f.replace(/\.json$/, '');
      map[ns] = JSON.parse(readFileSync(join(dir, f), 'utf8'));
    }
  }
  return map;
}
const packs = Object.fromEntries(Object.entries(localeDirs).map(([lang, dir]) => [lang, loadNamespaces(dir)]));

// 键存在:平铺点号键或嵌套路径任一命中
function keyExists(json, key) {
  if (json == null) return false;
  if (Object.prototype.hasOwnProperty.call(json, key)) return typeof json[key] === 'string';
  let node = json;
  for (const seg of key.split('.')) {
    if (node == null || typeof node !== 'object' || !Object.prototype.hasOwnProperty.call(node, seg)) return false;
    node = node[seg];
  }
  return typeof node === 'string';
}

// ---------- 源码收集 ----------
function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (name === 'locales' || name === 'generated' || name === 'node_modules') continue;
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, out);
    else if (/\.(tsx?|jsx?)$/.test(name)) out.push(p);
  }
  return out;
}
const files = walk(srcDir);

// const { t, t: tRoutes } = useTranslation('ns' | ['a','b'] | 空)
const USE_T = /const\s*\{([^}]+)\}\s*=\s*useTranslation\s*\(([^)]*)\)/g;
// 字面量键调用:tX('key') / tX("key")——名字前不能是单词字符/点/$(排除 split( 等误配)
const callRe = (name) => new RegExp(`(?:^|[^\\w$.])${name}\\(\\s*(['"])([^'"]+)\\1`, 'g');
// 带命名空间前缀的任意调用:t('ns:key') / i18next.t('ns:key') / i18n.t("ns:key")
const PREFIXED = /(?:^|[^\w$.])(?:i18next\.|i18n\.)?t\(\s*(['"])([\w-]+):([^'"]+)\1/g;

const missing = new Map(); // "lang|ns|key" -> refs[]
const unattributable = []; // 无默认 ns 的未前缀键(无法定位语言包)

const record = (lang, ns, key, ref) => {
  if (keyExists(packs[lang][ns], key)) return;
  const id = `${lang}|${ns}|${key}`;
  if (!missing.has(id)) missing.set(id, []);
  missing.get(id).push(ref);
};

for (const file of files) {
  const rel = relative(root, file).split(sep).join('/');
  const code = readFileSync(file, 'utf8');
  const refAt = (idx) => `${rel}:${code.slice(0, idx).split('\n').length}`;

  // 该文件里每个 t 绑定(含改名)对应的使用
  const bindings = [];
  for (const m of code.matchAll(USE_T)) {
    const nsArg = m[2].trim();
    let nsList = [];
    if (nsArg) {
      try {
        nsList = JSON.parse(nsArg.replace(/'/g, '"'));
        if (typeof nsList === 'string') nsList = [nsList];
      } catch {
        nsList = [];
      }
    }
    for (const piece of m[1].split(',')) {
      const b = piece.match(/^\s*(\w+)(?:\s*:\s*(\w+))?\s*$/);
      if (b && b[1] === 't') bindings.push({ local: b[2] || b[1], nsList, at: refAt(m.index) });
    }
  }

  for (const b of bindings) {
    for (const m of code.matchAll(callRe(b.local))) {
      const key = m[2];
      if (key.includes(':')) {
        const [ns, ...rest] = key.split(':');
        for (const lang of Object.keys(packs)) record(lang, ns, rest.join(':'), refAt(m.index));
      } else if (b.nsList.length) {
        for (const lang of Object.keys(packs)) {
          if (!b.nsList.some((ns) => keyExists(packs[lang][ns], key))) {
            record(lang, b.nsList[0], key, refAt(m.index));
          }
        }
      } else {
        unattributable.push(`${refAt(m.index)}  ${b.local}('${key}')`);
      }
    }
  }

  // ns:key 前缀形式(不依赖 useTranslation 解构)
  for (const m of code.matchAll(PREFIXED)) {
    for (const lang of Object.keys(packs)) record(lang, m[2], m[3], refAt(m.index));
  }
}

// ---------- 输出 ----------
let total = 0;
const byLang = {};
for (const [id, refs] of missing) {
  const [lang, ns, key] = id.split('|');
  byLang[lang] ??= {};
  byLang[lang][ns] ??= [];
  byLang[lang][ns].push([key, refs]);
  total += 1;
}

for (const [lang, nsMap] of Object.entries(byLang)) {
  console.log(`\n[${lang}]`);
  for (const [ns, keys] of Object.entries(nsMap)) {
    console.log(`  ${ns}.json 缺 ${keys.length} 键:`);
    for (const [key, refs] of keys) console.log(`    - ${key}    (${refs[0]}${refs.length > 1 ? ` 等 ${refs.length} 处` : ''})`);
  }
}

if (unattributable.length) {
  console.log(`\n[无法定位命名空间的键,仅提示] ${unattributable.length} 处`);
  for (const u of unattributable.slice(0, 10)) console.log(`  ${u}`);
}

if (total === 0) {
  console.log('\n✓ 两语言包无缺键');
} else {
  console.log(`\n✗ 共 ${total} 个键缺失(zh/en 合计)`);
  process.exit(1);
}
