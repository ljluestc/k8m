import { test, expect } from './fixtures'

const routes = ['/#/', '/#/NodeExec', '/#/PodExec', '/#/PodLog', '/#/MenuEditor', '/#/login']

async function assertNoNotFound(page: import('@playwright/test').Page) {
    const bodyText = await page.locator('body').innerText()
    // Common not found phrases to detect GH Pages or generic 404 content
    const patterns = [
        'Page Not Found',
        "The page you're looking for doesn't exist",
        '404',
    ]
    for (const p of patterns) {
        if (bodyText.includes(p)) {
            throw new Error(`Detected not-found marker in page: ${p}`)
        }
    }
}

test.describe('Route smoke - no 404', () => {
    for (const r of routes) {
        test(`route ${r} renders without not-found`, async ({ page }) => {
            // prevent silent page errors
            const errors: string[] = []
            page.on('pageerror', e => errors.push(String(e)))

            await page.goto(r)
            await page.waitForLoadState('domcontentloaded')
            await assertNoNotFound(page)
            expect(errors, `page errors on ${r}`).toEqual([])
        })
    }
})


