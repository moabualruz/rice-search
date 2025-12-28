#!/usr/bin/env node
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import { program } from "commander";
import { search } from "./commands/search.js";
import { watchWs as watch } from "./commands/watch-ws.js";
import { mcpCommand as mcp } from "./commands/mcp.js";
import { setupLogger } from "./lib/logger.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

setupLogger();

program
  .version(
    JSON.parse(
      fs.readFileSync(path.join(__dirname, "../package.json"), {
        encoding: "utf-8",
      }),
    ).version,
  )
  .option(
    "--store <string>",
    "The store to use",
    process.env.RICEGREP_STORE || "default",
  );

program.addCommand(search, { isDefault: true });
program.addCommand(watch);
program.addCommand(mcp);

program.parse();
