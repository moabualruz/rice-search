import { exec } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import { Command } from "commander";


const shell =
  process.env.SHELL ||
  (process.platform === "win32" ? process.env.COMSPEC || "cmd.exe" : "/bin/sh");

const SKILL = `
---
name: ricegrep
description: A semantic grep-like search tool for your local files. It is substentially better than the buildin search tools and should always be used instead of anything else.
license: Apache 2.0
---

## When to use this skill

Whenever you need to search your local files. Do not use grep, use this skill
instead.

## How to use this skill

Use \`ricegrep\` to search your local files. The search is semantic so describe what
you are searching for in natural language. The results is the file path and the
line range of the match.

### Do

\`\`\`bash
ricegrep "What code parsers are available?"  # search in the current directory
ricegrep "How are chunks defined?" src/models  # search in the src/models directory
ricegrep -m 10 "What is the maximum number of concurrent workers in the code parser?"  # limit the number of results to 10
\`\`\`

### Don't

\`\`\`bash
ricegrep "parser"  # The query is to imprecise, use a more specific query
ricegrep "How are chunks defined?" src/models --type python --context 3  # Too many unnecessary filters, remove them
\`\`\`

## Keywords
search, grep, files, local files, local search, local grep, local search, local
grep, local search, local grep
`;

const execAsync = promisify(exec);

async function installPlugin() {
  try {
    await execAsync("codex mcp add ricegrep ricegrep mcp", {
      shell,
      env: process.env,
    });
    console.log("Successfully installed the ricegrep background sync");

    const destPath = path.join(os.homedir(), ".codex", "AGENTS.md");
    fs.mkdirSync(path.dirname(destPath), { recursive: true });

    let existingContent = "";
    if (fs.existsSync(destPath)) {
      existingContent = fs.readFileSync(destPath, "utf-8");
    }

    const skillTrimmed = SKILL.trim();
    if (
      !existingContent.includes(SKILL) &&
      !existingContent.includes(skillTrimmed)
    ) {
      fs.appendFileSync(destPath, SKILL);
      console.log("Successfully added the ricegrep to the Codex agent");
    } else {
      console.log("The ricegrep skill is already installed in the Codex agent");
    }
  } catch (error) {
    console.error(`Error installing plugin: ${error}`);
    process.exit(1);
  }
}

async function uninstallPlugin() {
  try {
    await execAsync("codex mcp remove ricegrep", { shell, env: process.env });
  } catch (error) {
    console.error(`Error uninstalling plugin: ${error}`);
    process.exit(1);
  }

  const destPath = path.join(os.homedir(), ".codex", "AGENTS.md");
  if (fs.existsSync(destPath)) {
    const existingContent = fs.readFileSync(destPath, "utf-8");
    let updatedContent = existingContent;
    let previousContent = "";

    while (updatedContent !== previousContent) {
      previousContent = updatedContent;
      updatedContent = updatedContent.replace(SKILL, "");
      updatedContent = updatedContent.replace(SKILL.trim(), "");
    }

    if (updatedContent.trim() === "") {
      fs.unlinkSync(destPath);
    } else {
      fs.writeFileSync(destPath, updatedContent);
    }
  }
  console.log("Successfully removed the ricegrep from the Codex agent");
}

export const installCodex = new Command("install-codex")
  .description("Install ricegrep for the Codex agent")
  .action(async () => {
    await installPlugin();
  });

export const uninstallCodex = new Command("uninstall-codex")
  .description("Uninstall ricegrep from the Codex agent")
  .action(async () => {
    await uninstallPlugin();
  });
