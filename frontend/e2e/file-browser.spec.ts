import { test, expect } from '@playwright/test'

test.describe('File Browser', () => {
  test('desktop mode switch — toggle between terminal and file browser', async ({ page }) => {
    await page.goto('/')
    // Wait for at least one container to render
    await page.waitForSelector('[data-container-id]', { timeout: 10000 })

    // Find and click the file browser toggle button (FolderOpen icon)
    const toggleBtn = page.locator('[title="Browse files"]').first()
    await expect(toggleBtn).toBeVisible()
    await toggleBtn.click()

    // File tree should appear (search input)
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

    // Switch to file browser
    const toggleBtn = page.locator('[title="Browse files"]').first()
    await toggleBtn.click()

    // Wait for file tree to load
    await expect(page.locator('input[placeholder="Filter files..."]').first()).toBeVisible({ timeout: 5000 })

    // Look for a directory or file in the tree and click it
    // This is a smoke test — just verify the tree renders some content
    const treeItems = page.locator('.cursor-pointer')
    await expect(treeItems.first()).toBeVisible({ timeout: 5000 })
  })
})

test.describe('Terminal tabs', () => {
  test('close terminal returns to Agent tab', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('[data-container-id]', { timeout: 10000 })

    // Find the "+" button to create a new terminal
    const addBtn = page.locator('[title="New terminal"]').first()
    if (await addBtn.isVisible()) {
      await addBtn.click()

      // Wait for new terminal tab to appear
      await page.waitForTimeout(500)

      // Find and click the close button on the new terminal
      const closeBtn = page.locator('[role="button"]').first()
      if (await closeBtn.isVisible()) {
        await closeBtn.click()

        // Confirm close if dialog appears
        const confirmBtn = page.locator('text=Confirm').first()
        if (await confirmBtn.isVisible({ timeout: 1000 }).catch(() => false)) {
          await confirmBtn.click()
        }
      }

      // Agent tab should still be visible
      await expect(page.locator('text=Agent').first()).toBeVisible()
    }
  })
})

test.describe('Mobile file browser', () => {
  test.use({ viewport: { width: 375, height: 812 } })

  test('mobile mode switch and navigation', async ({ page }) => {
    await page.goto('/')
    await page.waitForTimeout(2000)

    // Find file browser toggle button
    const toggleBtn = page.locator('[title="Browse files"]').first()
    if (await toggleBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await toggleBtn.click()

      // File tree should appear
      await expect(page.locator('input[placeholder="Filter files..."]').first()).toBeVisible({ timeout: 5000 })

      // Click on a file entry if available
      const fileEntry = page.locator('.cursor-pointer').first()
      if (await fileEntry.isVisible({ timeout: 3000 }).catch(() => false)) {
        await fileEntry.click()
        await page.waitForTimeout(500)

        // If we navigated to file preview, back button should exist
        const backBtn = page.locator('[title="Back to terminal"]')
        if (await backBtn.isVisible({ timeout: 1000 }).catch(() => false)) {
          await backBtn.click()
        }
      }
    }
  })
})
