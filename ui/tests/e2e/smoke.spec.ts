import { test, expect } from './fixtures'

test('login page renders', async ({ page }) => {
    await page.goto('/#/login')
    await expect(page.getByRole('heading', { name: '欢迎登录' })).toBeVisible()
    await expect(page.getByPlaceholder('请输入用户名')).toBeVisible()
    await expect(page.getByPlaceholder('请输入密码')).toBeVisible()
})


