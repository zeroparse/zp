// Run the embedded TypeScript literal matcher without rule/input setup or trees.
// Requires a Node.js version that runs TypeScript directly, and Go to generate the library.
// The library is the lib.ts that `zp gen -lang ts -lib only` writes: the lib.ts source of the tree
// holds placeholder methods and lacks budget.ts, trace.ts and profile.ts, which generation fills in
// and appends. An optional path selects another generated lib.ts for before/after comparisons.
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

function generatedLibrary() {
  const out = mkdtempSync(join(tmpdir(), "zp-ts-lib-"));
  try {
    const root = fileURLToPath(new URL("..", import.meta.url));
    const gen = spawnSync("go", ["run", "./cmd/zp", "gen", "-lang", "ts", "-lib", "only", "-o", out], { cwd: root, stdio: "inherit" });
    if (gen.error) throw gen.error;
    if (gen.status !== 0) throw new Error("zp gen failed");
    return readFileSync(join(out, "lib.ts"), "utf8");
  } finally {
    rmSync(out, { recursive: true, force: true });
  }
}
const source = process.argv[2] ? readFileSync(process.argv[2], "utf8") : generatedLibrary();
const helper = String.raw`
export function benchmarkLiteralMatchers() {
 for(const text of ["keyword","日本語","�","é�x"]){
  const chars=Array.from(text);
  const lit:Lit={text,cps:chars.map(x=>x.codePointAt(0)!),bytes:Array.from(encodeUTF8(text))};
  for(const match of [true,false]){
   const input=encodeUTF8(match?text:chars.slice(0,-1).join("")+"!");
   for(const unit of [Bytes,CodePoints]){
    const p=new Parser(mkGrammar(100_000,false,false),input,unit,0,100_000);p.silent=1;
    const once=()=>{p.pos=0;const got=p.matchLiteral(lit,0,false)!==undefined;if(got!==match)throw new Error("incorrect match")};
    for(let i=0;i<100000;i++)once();
    let count=0;const begin=performance.now();let elapsed=0;
    do{for(let i=0;i<10000;i++)once();count+=10000;elapsed=performance.now()-begin;}while(elapsed<200);
    console.log(JSON.stringify({text,match,unit,ns:elapsed*1e6/count}));
   }
  }
 }
}
benchmarkLiteralMatchers();
`;
const dir = mkdtempSync(join(tmpdir(), "zp-ts-literal-"));
try {
  const path = join(dir, "matcher.ts");
  writeFileSync(path, source + helper);
  const result = spawnSync(process.execPath, [path], { stdio: "inherit" });
  if (result.error) throw result.error;
  process.exitCode = result.status ?? 1;
} finally {
  rmSync(dir, { recursive: true, force: true });
}
