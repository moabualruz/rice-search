import { test, expect } from '@playwright/test';
import { spawn } from 'child_process';
import path from 'path';

const binaryName = process.platform === 'win32' ? 'rice-search.exe' : 'rice-search';
const binaryPath = path.join(process.cwd(), '..', '..', binaryName);

test.describe('CLI Server', () => {
    test('should start and accept connections', async () => {
        // This test starts the server, waits for it to be ready, checks health, then kills it.
        // Note: In a real CI, we might skip this if the test runner assumes a running server.
        // But the requirement is to verify CLI actions including server startup.
        
        // We use a different port to avoid conflict with the main test server if running in parallel
        // strictly speaking, but for simplicity let's assume we test the binary capability here.
        const port = 8082; 
        
        console.log(`Starting server on port ${port}...`);
        const server = spawn(binaryPath, ['serve', '--port', port.toString()], {
            env: { ...process.env, RICE_LOG_LEVEL: 'info' } // Ensure clean env
        });

        const waitForServer = new Promise<void>((resolve, reject) => {
            let buffer = '';
            server.stdout.on('data', (data) => {
                const output = data.toString();
                buffer += output;
                console.log('Server Output:', output);
                if (output.includes('Server started') || output.includes('Listening on')) {
                    resolve();
                }
            });
            
            server.stderr.on('data', (data) => console.error('Server Error:', data.toString()));
            
            server.on('error', (err) => reject(err));
            
            // Timeout
            setTimeout(() => {
                if (!buffer.includes('Server started')) {
                    console.log('Server buffer:', buffer);
                    reject(new Error('Server start timeout'));
                }
            }, 10000);
        });

        try {
            await waitForServer;
            
            // Allow a moment for HTTP listener
            await new Promise(r => setTimeout(r, 1000));
            
            // Check health
            const response = await fetch(`http://localhost:${port}/health`);
            expect(response.status).toBe(200);
            
        } finally {
            console.log('Killing server...');
            server.kill();
        }
    });
});
