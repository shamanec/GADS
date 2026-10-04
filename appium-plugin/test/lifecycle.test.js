import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

// Exercise the shipped plugin with only its external imports replaced. No
// Appium installation or physical device is needed for callback failure tests.
function fixture() {
    const source = readFileSync(new URL('../gads-appium-plugin.js', import.meta.url), 'utf8')
        .replace(/^import .*;$/gm, '')
        .replace('export { GadsAppium };', 'globalThis.Plugin = GadsAppium;');
    const context = vm.createContext({BasePlugin: class {}, logger: {getLogger: () => ({debug() {}, warn() {}})}});
    vm.runInContext(source, context);
    const Plugin = context.Plugin;
    Plugin.cfg = {udid: 'device-a'};
    const sessions = new Set();
    const removed = [];
    Plugin.apiClient = {async addSession() {}, async removeSession(id) {removed.push(id);}};
    const driver = {async deleteSession(id) {sessions.delete(id); return {value: null};}};
    const create = async (id) => {
        const plugin = new Plugin();
        await plugin.createSession(async () => {sessions.add(id); return {value: [id, {}]};}, driver);
        return plugin;
    };
    return {Plugin, driver, sessions, removed, create};
}

test('normal sequential sessions use Appium command chain for cleanup', async () => {
    const f = fixture();
    const a = await f.create('a');
    await a.deleteSession(() => f.driver.deleteSession('a'), f.driver, 'a');
    assert.equal(f.sessions.size, 0);
    await f.create('b');
    assert.deepEqual([...f.sessions], ['b']);
    assert.deepEqual(f.removed, ['a']);
});

test('failed creation after driver allocation rolls back before B starts', async () => {
    const f = fixture();
    f.Plugin.apiClient.addSession = async () => {throw new Error('provider unavailable');};
    await assert.rejects(f.create('a'), /provider unavailable/);
    assert.equal(f.sessions.size, 0);
    assert.equal(f.Plugin.currentSessionId, '');
    f.Plugin.apiClient.addSession = async () => {};
    await f.create('b');
    assert.deepEqual([...f.sessions], ['b']);
});

test('driver creation error is preserved and subsequent creation works', async () => {
    const f = fixture();
    await assert.rejects(new f.Plugin().createSession(async () => {throw new Error('XCUITest failed');}, f.driver), /XCUITest failed/);
    await f.create('b');
    assert.deepEqual([...f.sessions], ['b']);
});

test('late unexpected shutdown of A cannot clear B', async () => {
    const f = fixture();
    const a = await f.create('a');
    await f.driver.deleteSession('a'); // Appium owns unexpected-shutdown teardown
    await f.create('b');
    await a.onUnexpectedShutdown(f.driver, new Error('WDA unavailable'));
    assert.equal(f.Plugin.currentSessionId, 'b');
    assert.deepEqual(f.removed, ['a']);
});

test('notification failure cannot turn completed DELETE into a failed command', async () => {
    const f = fixture();
    const a = await f.create('a');
    f.Plugin.apiClient.removeSession = async () => {throw new Error('provider unavailable');};
    await a.deleteSession(() => f.driver.deleteSession('a'), f.driver, 'a');
    assert.equal(f.sessions.size, 0);
    await f.create('b');
    assert.equal(f.Plugin.currentSessionId, 'b');
});
