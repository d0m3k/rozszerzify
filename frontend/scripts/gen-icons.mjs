// gen-icons.mjs — generate PWA icons (PNG) with zero dependencies.
// Draws a cute baby-food jar with a tiny spoon... well, three food dots.
// Pure Node: zlib for PNG compression + manual PNG chunks + CRC32.
import { deflateSync } from 'node:zlib';
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const outDir = join(__dirname, '..', 'public');
mkdirSync(outDir, { recursive: true });

/* ---------- PNG encoder ---------- */
const crcTable = new Int32Array(256);
for (let n = 0; n < 256; n++) {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  crcTable[n] = c;
}
function crc32(buf) {
  let crc = -1;
  for (let i = 0; i < buf.length; i++) crc = (crc >>> 8) ^ crcTable[(crc ^ buf[i]) & 0xff];
  return (crc ^ -1) >>> 0;
}
function chunk(type, data) {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
  const typeBuf = Buffer.from(type, 'ascii');
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([typeBuf, data])));
  return Buffer.concat([len, typeBuf, data, crc]);
}
function encodePNG(size, rgba) {
  const sig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(size, 0); ihdr.writeUInt32BE(size, 4);
  ihdr[8] = 8; ihdr[9] = 6;
  const rowBytes = size * 4;
  const raw = Buffer.alloc((rowBytes + 1) * size);
  for (let y = 0; y < size; y++) {
    raw[y * (rowBytes + 1)] = 0;
    rgba.copy(raw, y * (rowBytes + 1) + 1, y * rowBytes, (y + 1) * rowBytes);
  }
  const idat = deflateSync(raw, { level: 9 });
  return Buffer.concat([sig, chunk('IHDR', ihdr), chunk('IDAT', idat), chunk('IEND', Buffer.alloc(0))]);
}

/* ---------- geometry helpers (normalized 0..1 coords) ---------- */
function rrectSDF(px, py, cx, cy, hw, hh, r) {
  const qx = Math.abs(px - cx) - (hw - r);
  const qy = Math.abs(py - cy) - (hh - r);
  const ax = Math.max(qx, 0), ay = Math.max(qy, 0);
  return Math.hypot(ax, ay) + Math.min(Math.max(qx, qy), 0) - r;
}
function circleSDF(px, py, cx, cy, r) {
  return Math.hypot(px - cx, py - cy) - r;
}
const lerp = (a, b, t) => a + (b - a) * t;

/* ---------- color helpers ---------- */
function hexRGB(h) {
  return [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];
}
function mix(c1, c2, t) {
  return [lerp(c1[0], c2[0], t), lerp(c1[1], c2[1], t), lerp(c1[2], c2[2], t)];
}

/* ---------- artwork ---------- */
// Ordered list of shapes: { kind: 'rrect'|'circle', ... , color, overlap?: sdf subtract }
function shapes() {
  return [
    { kind: 'rrect', cx: 0.5, cy: 0.66, hw: 0.20, hh: 0.22, r: 0.065, color: '#FFF8F0' },       // jar body
    { kind: 'rrect', cx: 0.5, cy: 0.415, hw: 0.205, hh: 0.07, r: 0.045, color: '#45B5AC' },    // lid
    { kind: 'rrect', cx: 0.5, cy: 0.30, hw: 0.075, hh: 0.028, r: 0.02, color: '#3A9A92' },     // knob
    { kind: 'circle', cx: 0.37, cy: 0.60, r: 0.052, color: '#7BC950' },                       // dot green
    { kind: 'circle', cx: 0.50, cy: 0.72, r: 0.052, color: '#FF8E72' },                       // dot orange
    { kind: 'circle', cx: 0.63, cy: 0.60, r: 0.052, color: '#FF6B6B' },                       // dot coral
    { kind: 'circle', cx: 0.50, cy: 0.885, r: 0.035, color: '#C97B2D' },                      // shadow under jar
  ];
}

function render(size, ss = 4) {
  const buf = Buffer.alloc(size * size * 4);
  const top = hexRGB('#FFE9D6');
  const bottom = hexRGB('#FFCEA6');
  const items = shapes();

  const coverage = (x, y) => {
    let inside = 0;
    for (const s of items) {
      const d = s.kind === 'rrect'
        ? rrectSDF(x, y, s.cx, s.cy, s.hw, s.hh, s.r)
        : circleSDF(x, y, s.cx, s.cy, s.r);
      if (d < 0) inside = 1;
    }
    return inside;
  };

  for (let py = 0; py < size; py++) {
    for (let px = 0; px < size; px++) {
      // supersample
      let acc = 0, r = 0, g = 0, b = 0;
      for (let sy = 0; sy < ss; sy++) {
        for (let sx = 0; sx < ss; sx++) {
          const x = (px + (sx + 0.5) / ss) / size;
          const y = (py + (sy + 0.5) / ss) / size;
          const cov = coverage(x, y);
          acc += cov;
          if (cov > 0) {
            // shape color: find last shape covering the point
            let col = top;
            for (const s of items) {
              const d = s.kind === 'rrect'
                ? rrectSDF(x, y, s.cx, s.cy, s.hw, s.hh, s.r)
                : circleSDF(x, y, s.cx, s.cy, s.r);
              if (d < 0) col = hexRGB(s.color);
            }
            if (sx === 0 && sy === 0) { /* nop */ }
            r += col[0]; g += col[1]; b += col[2];
          }
        }
      }
      const total = ss * ss;
      let alpha = acc / total;
      let fr, fg, fb;
      if (alpha === 0) {
        // background gradient
        const t = py / size;
        const c = mix(top, bottom, t);
        fr = c[0]; fg = c[1]; fb = c[2];
        alpha = 1;
      } else {
        // average shape color (rough, background underneath handled by ~full coverage)
        fr = r / acc; fg = g / acc; fb = b / acc;
      }
      const o = py * size * 4 + px * 4;
      buf[o] = Math.round(fr); buf[o + 1] = Math.round(fg); buf[o + 2] = Math.round(fb); buf[o + 3] = Math.round(alpha * 255);
    }
  }
  return buf;
}

/* ---------- write files ---------- */
for (const [name, size] of [['icon-192', 192], ['icon-512', 512]]) {
  writeFileSync(join(outDir, `${name}.png`), encodePNG(size, render(size)));
  writeFileSync(join(outDir, `${name}-maskable.png`), encodePNG(size, render(size)));
  console.log(`${name}.png + maskable (${size}x${size}) ✓`);
}

// SVG fallback + favicon
const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <defs><linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
    <stop offset="0" stop-color="#FFE9D6"/><stop offset="1" stop-color="#FFCEA6"/>
  </linearGradient></defs>
  <rect width="512" height="512" fill="url(#bg)"/>
  <rect x="154" y="235" width="204" height="225" rx="33" fill="#FFF8F0"/>
  <rect x="151" y="180" width="210" height="72" rx="23" fill="#45B5AC"/>
  <rect x="217" y="130" width="78" height="29" rx="14" fill="#3A9A92"/>
  <circle cx="190" cy="307" r="27" fill="#7BC950"/>
  <circle cx="256" cy="369" r="27" fill="#FF8E72"/>
  <circle cx="322" cy="307" r="27" fill="#FF6B6B"/>
</svg>`;
writeFileSync(join(outDir, 'favicon.svg'), svg);
console.log('favicon.svg + icon svg ✓');