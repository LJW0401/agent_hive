import { test, expect } from '@playwright/test'

test('homepage loads', async ({ page }) => {
  await page.goto('/')
  // The page should load and show the app
  await expect(page.locator('body')).toBeVisible()
})
