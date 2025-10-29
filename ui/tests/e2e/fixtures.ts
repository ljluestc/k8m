import { test as base, expect } from '@playwright/test'
import fs from 'fs'
import path from 'path'

export const test = base.extend({
    // no custom fixtures
})

test.afterEach(async ({ page }, testInfo) => {
    try {
        const coverage = await page.evaluate(() => (window as any).__coverage__)
        if (coverage) {
            const outDir = path.resolve(process.cwd(), '.nyc_output')
            fs.mkdirSync(outDir, { recursive: true })
            const safeName = testInfo.title.replace(/[^a-z0-9]/gi, '_').toLowerCase()
            fs.writeFileSync(path.join(outDir, `${safeName}.json`), JSON.stringify(coverage))
        }
    } catch {
        // ignore if page is closed
    }
})

export { expect }


