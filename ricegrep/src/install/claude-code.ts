import { Command } from "commander";

async function installPlugin() {
  console.log(`
📦 ricegrep Claude Code Setup Instructions

Since ricegrep uses a local Rice Search backend instead of a marketplace plugin, 
you'll need to manually configure Claude Code to use ricegrep:

1. Ensure ricegrep is installed globally:
   npm install -g ricegrep

2. Start your Rice Search platform:
   cd rice-search
   docker-compose up -d

3. In your project directory, start ricegrep watch:
   cd your-project
   ricegrep watch

4. Use ricegrep directly in your Claude Code sessions:
   - Run: ricegrep "your search query"
   - Or: ricegrep "auth middleware" src/

Claude Code can now use ricegrep commands directly for semantic code search!
  `);
}

async function uninstallPlugin() {
  console.log(`
🗑️ ricegrep Claude Code Removal

Since ricegrep is installed as a global CLI tool, to remove it:

1. Stop any running ricegrep watch processes in your projects
2. Uninstall the global package:
   npm uninstall -g ricegrep
3. Stop your Rice Search platform if no longer needed:
   cd rice-search
   docker-compose down

ricegrep has been removed from your system.
  `);
}

export const installClaudeCode = new Command("install-claude-code")
  .description("Setup ricegrep for Claude Code")
  .action(async () => {
    await installPlugin();
  });

export const uninstallClaudeCode = new Command("uninstall-claude-code")
  .description("Remove ricegrep from your system")
  .action(async () => {
    await uninstallPlugin();
  });
