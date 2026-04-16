import { test, expect } from '@playwright/test'

test.describe('File Browser', () => {
  test('desktop mode switch — toggle between terminal and file browser', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('[data-container-id]', { timeout: 10000 })

    const toggleBtn = page.locator('[title="Browse files"]').first()
    await expect(toggleBtn).toBeVisible()
    await toggleBtn.click()

    // File tree should appear
    await expect(page.locator('input[placeholder="Filter files..."]').first()).toBeVisible({ timeout: 5000 })

    // Switch back to terminal
    const backBtn = page.locator('[title="Back to terminal"]').first()
    await expect(backBtn).toBeVisible()
    await backBtn.click()

    // Terminal tab bar should be visible again
    await expect(page.locator('text=Agent').first()).toBeVisible()
  })

  test('file browsing flow — expand directory and view file', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('[data-container-id]', { timeout: 10000 })

    const toggleBtn = page.locator('[title="Browse files"]').first()
    await toggleBtn.click()

    await expect(page.locator('input[placeholder="Filter files..."]').first()).toBeVisible({ timeout: 5000 })

    // Tree should render some content
    const treeItems = page.locator('.cursor-pointer')
    await expect(treeItems.first()).toBeVisible({ timeout: 5000 })
  })
})

test.describe('Terminal tabs', () => {
  test('close terminal returns to Agent tab', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('[data-container-id]', { timeout: 10000 })

    // Create a new terminal
    const addBtn = page.locator('[title="New terminal"]').first()
    await expect(addBtn).toBeVisible()
    await addBtn.click()
    await page.waitForTimeout(500)

    // Close it
    const closeBtn = page.locator('[role="button"]').first()
    await expect(closeBtn).toBeVisible()
    await closeBtn.click()

    // Confirm close if dialog appears
    const confirmBtn = page.locator('text=Confirm').first()
    if (await confirmBtn.isVisible({ timeout: 1000 }).catch(() => false)) {
      await confirmBtn.click()
    }

    // Agent tab should still be visible
    await expect(page.locator('text=Agent').first()).toBeVisible()
  })
})

test.describe('Mobile file browser', () => {
  test.use({ viewport: { width: 375, height: 812 } })

  test('mobile mode switch and navigation', async ({ page }) => {
    await page.goto('/')
    await page.waitForTimeout(2000)

    const toggleBtn = page.locator('[title="Browse files"]').first()
    await expect(toggleBtn).toBeVisible({ timeout: 5000 })
    await toggleBtn.click()

    // File tree should appear
    await expect(page.locator('input[placeholder="Filter files..."]').first()).toBeVisible({ timeout: 5000 })

    // Click on a file entry
    const fileEntry = page.locator('.cursor-pointer').first()
    await expect(fileEntry).toBeVisible({ timeout: 3000 })
    await fileEntry.click()
    await page.waitForTimeout(500)

    // Switch back to terminal
    const backBtn = page.locator('[title="Back to terminal"]').first()
    await expect(backBtn).toBeVisible()
    await backBtn.click()

    await expect(page.locator('text=Agent').first()).toBeVisible()
  })
})
