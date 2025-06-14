import { test, expect } from '@playwright/test';

test('open settings modal', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: /connect/i })).toBeVisible();
  await page.getByRole('button', { name: /settings/i }).click();
  await expect(page.locator('h2', { hasText: 'Settings' })).toBeVisible();
  await page.getByRole('button', { name: /close/i }).click();
  await expect(page.locator('h2', { hasText: 'Settings' })).toBeHidden();
});
