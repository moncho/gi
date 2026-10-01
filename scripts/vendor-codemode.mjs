// Refreshes internal/codemode/vendor from the installed Pi packages:
//   quickjs.wasm  - quickjs-wasi (MIT, Vercel), QuickJS-ng compiled to WASI
//   prelude.js    - Pi's codemode prelude (MIT, @earendil-works/pi-codemode)
// Run: bun scripts/vendor-codemode.mjs
import { copyFileSync, readFileSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';

const nm = process.env.PI_NODE_MODULES || `${homedir()}/.bun/install/global/node_modules`;
const out = 'internal/codemode/vendor';
const piPkg = JSON.parse(readFileSync(`${nm}/@earendil-works/pi-codemode/package.json`, 'utf8'));
const qjsPkg = JSON.parse(readFileSync(`${nm}/quickjs-wasi/package.json`, 'utf8'));
const { PRELUDE_SOURCE, MAX_STORE_VALUE_CHARS, MAX_STORE_TOTAL_CHARS } = await import(`${nm}/@earendil-works/pi-codemode/dist/runtime/prelude-source.js`);

copyFileSync(`${nm}/quickjs-wasi/quickjs.wasm`, `${out}/quickjs.wasm`);
copyFileSync(`${nm}/quickjs-wasi/LICENSE`, `${out}/LICENSE.quickjs-wasi`);
writeFileSync(`${out}/prelude.js`, `// Vendored from @earendil-works/pi-codemode ${piPkg.version} (MIT) by scripts/vendor-codemode.mjs; do not edit.\n${PRELUDE_SOURCE}\n`);
writeFileSync(`${out}/VERSIONS`, `pi-codemode ${piPkg.version} (${piPkg.license})\nquickjs-wasi ${qjsPkg.version} (${qjsPkg.license})\nMAX_STORE_VALUE_CHARS ${MAX_STORE_VALUE_CHARS}\nMAX_STORE_TOTAL_CHARS ${MAX_STORE_TOTAL_CHARS}\n`);
writeFileSync(`${out}/LICENSE.pi-codemode`, `MIT License\n\nCopyright (c) the pi authors (https://github.com/earendil-works/pi)\n\nPermission is hereby granted, free of charge, to any person obtaining a copy\nof this software and associated documentation files (the "Software"), to deal\nin the Software without restriction, including without limitation the rights\nto use, copy, modify, merge, publish, distribute, sublicense, and/or sell\ncopies of the Software, and to permit persons to whom the Software is\nfurnished to do so, subject to the following conditions:\n\nThe above copyright notice and this permission notice shall be included in all\ncopies or substantial portions of the Software.\n\nTHE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR\nIMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,\nFITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE\nAUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\nLIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\nOUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE\nSOFTWARE.\n`);
console.log(`vendored pi-codemode ${piPkg.version}, quickjs-wasi ${qjsPkg.version}`);
