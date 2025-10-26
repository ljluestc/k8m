import { test, expect } from '@playwright/test'

test('login flow with mocked backend', async ({ page }) => {
    await page.route('**/auth/sso/config', async route => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ status: 0, data: [] }),
        })
    })
    await page.route('**/auth/ldap/config', async route => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ status: 0, data: { enabled: false } }),
        })
    })
    await page.route('**/auth/login', async route => {
        expect(route.request().method()).toBe('POST')
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ token: 'test-token' }),
        })
    })

    await page.goto('/#/login')
    await expect(page.getByRole('heading', { name: '欢迎登录' })).toBeVisible()
    await page.getByPlaceholder('请输入用户名').fill('user')
    await page.getByPlaceholder('请输入密码').fill('pass')
    await page.getByRole('button', { name: '登 录' }).click()

    // Token saved to localStorage
    const token = await page.evaluate(() => localStorage.getItem('token'))
    expect(token).toBe('test-token')
})


