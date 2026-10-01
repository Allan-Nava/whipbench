#!/usr/bin/env node
// leakcheck.mjs — nothing private in the tracked files, nor anywhere in the history.
//
// A public repository keeps every commit readable on the forge, so a forward fix of a
// leak is not a fix. This runs over the working tree's tracked files *and* over
// `git log --all -p` (diffs, messages, author and committer lines), and fails on:
//
//   - absolute home-directory paths and file URLs
//   - e-mail addresses other than the author's public one, GitHub's noreply
//     addresses and the reserved example domains
//   - private IPv4 addresses (10/8, 172.16/12, 192.168/16)
//   - strings shaped like credentials (GitHub, AWS, Slack, Google, Anthropic keys,
//     PEM private keys)
//   - tool-attribution trailers or footers in commits and files
//   - any name on a private denylist
//
// The denylist is deliberately not in this repository — a list of names that must
// not appear is itself a list of those names. It is read, one name per line, from
// the LEAKCHECK_DENYLIST environment variable or the file LEAKCHECK_DENYLIST_FILE
// names; CI passes a repository secret of that name when one is configured, and a
// missing list is a notice, not a failure. Names match case-insensitively, also
// across `-`, `_` and `.` (so "a-b" on the list catches "A_B" and "ab").
//
//   node scripts/leakcheck.mjs            tracked files and full history
//   node scripts/leakcheck.mjs --files    tracked files only
//
// Node 18+, no dependencies. The patterns below are assembled from pieces so this
// file does not match itself.

import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const ALLOWED_EMAILS = new Set(['allannava95@gmail.com', 'noreply@github.com'])
const ALLOWED_EMAIL_DOMAINS = [/\.?users\.noreply\.github\.com$/, /(^|\.)example\.(com|org|net)$/, /\.test$/, /\.example$/, /\.invalid$/]

const j = (...p) => p.join('')
export const RULES = [
  { name: 'home path', re: new RegExp(j('/(', 'Users', '|', 'home', ')/[A-Za-z0-9._-]+/'), 'g') },
  { name: 'file URL', re: new RegExp(j('file', ':', '//'), 'gi') },
  { name: 'private IPv4', re: /\b(?:10\.\d{1,3}|192\.168|172\.(?:1[6-9]|2\d|3[01]))\.\d{1,3}\.\d{1,3}\b/g },
  { name: 'GitHub token', re: new RegExp(j('\\b(?:gh[pousr]', '_[A-Za-z0-9]{30,}|github', '_pat_[A-Za-z0-9_]{30,})'), 'g') },
  { name: 'AWS key', re: new RegExp(j('\\bAK', 'IA[0-9A-Z]{16}\\b'), 'g') },
  { name: 'Slack token', re: new RegExp(j('\\bxox', '[abprs]-[A-Za-z0-9-]{10,}'), 'g') },
  { name: 'Google key', re: new RegExp(j('\\bAI', 'za[0-9A-Za-z_-]{35}'), 'g') },
  { name: 'Anthropic key', re: new RegExp(j('\\bsk-', 'ant-[A-Za-z0-9_-]{20,}'), 'g') },
  { name: 'private key', re: new RegExp(j('-----BEGIN [A-Z ]*PRIV', 'ATE KEY-----'), 'g') },
  { name: 'attribution trailer', re: new RegExp(j('co-', 'authored-by:.*(?:clau', 'de|anthro', 'pic)'), 'gi') },
  { name: 'attribution footer', re: new RegExp(j('gener', 'ated with \\[?clau', 'de'), 'gi') },
]

const EMAIL = /[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}/g

export function emailFindings(line) {
  const out = []
  for (const m of line.matchAll(EMAIL)) {
    const addr = m[0].toLowerCase()
    const domain = addr.split('@')[1]
    if (ALLOWED_EMAILS.has(addr) || ALLOWED_EMAIL_DOMAINS.some((d) => d.test(domain))) continue
    out.push({ rule: 'e-mail address', match: m[0] })
  }
  return out
}

// Compound words — runs of letters and digits joined by - _ or . — lower-cased
// with the joins removed, so a listed name is found however it is spelt.
const squash = (s) => s.toLowerCase().replace(/[-_.]/g, '')
export function nameFindings(line, names) {
  if (!names.length) return []
  const out = []
  for (const m of line.matchAll(/[A-Za-z0-9]+(?:[-_.][A-Za-z0-9]+)*/g)) {
    const word = squash(m[0])
    for (const n of names) if (word.includes(n)) out.push({ rule: 'denylisted name', match: '(a name on the private denylist)' })
  }
  return out
}

export function scanLine(line, names = []) {
  const out = []
  for (const r of RULES) for (const m of line.matchAll(r.re)) out.push({ rule: r.name, match: m[0] })
  return [...out, ...emailFindings(line), ...nameFindings(line, names)]
}

export function loadNames(env = process.env) {
  let raw = env.LEAKCHECK_DENYLIST ?? ''
  if (env.LEAKCHECK_DENYLIST_FILE && existsSync(env.LEAKCHECK_DENYLIST_FILE)) raw += '\n' + readFileSync(env.LEAKCHECK_DENYLIST_FILE, 'utf8')
  return [...new Set(raw.split('\n').map((l) => l.trim()).filter((l) => l && !l.startsWith('#')).map(squash))]
}

const git = (args) => execFileSync('git', args, { cwd: ROOT, encoding: 'utf8', maxBuffer: 1 << 30 })

function scanFiles(names) {
  const hits = []
  const files = git(['ls-files', '-z']).split('\0').filter(Boolean)
  for (const f of files) {
    let text
    try {
      const buf = readFileSync(resolve(ROOT, f))
      if (buf.includes(0)) continue // binary: the clips
      text = buf.toString('utf8')
    } catch {
      continue
    }
    text.split('\n').forEach((line, i) => {
      for (const h of scanLine(line, names)) hits.push(`${f}:${i + 1}: ${h.rule}: ${h.match}`)
    })
  }
  return { hits, count: files.length }
}

function scanHistory(names) {
  const hits = []
  let commits = 0
  let sha = ''
  const log = git(['log', '--all', '-p', '--no-color', '--text', '--format=commit %H%nauthor %an <%ae>%ncommitter %cn <%ce>%n%n%B'])
  for (const line of log.split('\n')) {
    if (line.startsWith('commit ') && /^commit [0-9a-f]{40}$/.test(line)) {
      sha = line.slice(7, 19)
      commits++
      continue
    }
    for (const h of scanLine(line, names)) hits.push(`${sha}: ${h.rule}: ${h.match}`)
  }
  return { hits, commits }
}

function main() {
  const names = loadNames()
  if (!names.length) console.log('notice: no private denylist (LEAKCHECK_DENYLIST / LEAKCHECK_DENYLIST_FILE) — names are not checked, everything else is')
  const files = scanFiles(names)
  const hits = [...files.hits]
  let summary = `${files.count} tracked files`
  if (!process.argv.includes('--files')) {
    const hist = scanHistory(names)
    hits.push(...hist.hits.map((h) => `history ${h}`))
    summary += `, ${hist.commits} commits`
  }
  const unique = [...new Set(hits)]
  if (unique.length) {
    for (const h of unique.slice(0, 200)) console.error(h)
    console.error(`\nleakcheck: ${unique.length} finding(s) over ${summary}. A finding in history needs a rewrite before pushing, not a forward fix.`)
    process.exit(1)
  }
  console.log(`ok — nothing private in ${summary}${names.length ? `, ${names.length} denylisted names checked` : ''}`)
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
