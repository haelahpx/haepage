// Validates the example config and built-in theme manifests against the
// published JSON Schemas. Run from CI via `npm --prefix web run validate`.
import Ajv2020 from "ajv/dist/2020.js";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");

const checks = [
  ["schema/haepage.schema.json", ["haepage.example.json"]],
  ["schema/theme.schema.json", ["themes/android/theme.json", "themes/apple/theme.json"]],
  ["schema/widget.schema.json", []],
  ["schema/known-apps.schema.json", []],
];

const readJSON = async (path) => JSON.parse(await readFile(join(root, path), "utf8"));

const ajv = new Ajv2020({ allErrors: true, strict: "log" });
let failed = false;

for (const [schemaPath, files] of checks) {
  // Compiling every schema also catches mistakes in schemas with no files yet.
  const validate = ajv.compile(await readJSON(schemaPath));
  for (const file of files) {
    if (validate(await readJSON(file))) {
      console.log(`ok    ${file}`);
    } else {
      failed = true;
      console.error(`FAIL  ${file}`);
      for (const err of validate.errors) console.error(`      ${err.instancePath || "/"} ${err.message}`);
    }
  }
}

if (failed) process.exit(1);
