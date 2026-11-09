import { test, expect } from '@playwright/test';

test('登记、核验、筛选、删除形成完整闭环', async ({ page }, info) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('/');
  await expect(page.locator('#connection')).toHaveText('服务已连接');
  await expect(page.locator('#mode-description')).toContainText('不会执行身份识别');
  await expect(page.locator('#verify-submit')).toBeDisabled();
  await page.getByRole('button', { name: /人员登记/ }).click();
  await page.getByRole('button', { name: /使用示例图/ }).click();
  await page.getByLabel('姓名或演示名称').fill(`教学人员 ${info.project.name}`);
  await expect(page.locator('#enroll-submit')).toBeDisabled();
  await page.locator('#enroll-consent').check();
  await page.getByRole('button', { name: /保存人员/ }).click();
  await expect(page.locator('#page-title')).toHaveText('让每一次核验，更清晰。');
  await expect(page.locator('#subject')).not.toHaveValue('');
  await page.locator('#verify-consent').check();
  await page.getByRole('button', { name: /开始核验/ }).click();
  await expect(page.locator('#result-title')).toHaveText('演示完成');
  await expect(page.locator('#result-detail')).toHaveText('未生成身份相似度分数');
  await expect(page.locator('#toast')).toBeHidden();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: info.outputPath('verification.png'), fullPage: true });
  await page.getByRole('button', { name: /核验记录/ }).click();
  await expect(page.locator('#record-list')).toContainText(`教学人员 ${info.project.name}`);
  await page.getByLabel('筛选核验结果').selectOption('matched');
  await expect(page.getByText('暂无符合条件的记录')).toBeVisible();
  await page.getByLabel('筛选核验结果').selectOption('demo');
  await expect(page.locator('.record-row')).toHaveCount(1);
  await page.getByRole('button', { name: /人员登记/ }).click();
  await page.getByRole('button', { name: `删除 教学人员 ${info.project.name}`, exact: true }).click();
  await page.getByRole('button', { name: '确认删除', exact: true }).click();
  await expect(page.locator('.person-row')).toHaveCount(0);
  await page.getByRole('button', { name: /核验记录/ }).click();
  await expect(page.getByText('暂无符合条件的记录')).toBeVisible();
  expect(errors).toEqual([]);
});

test('摄像头权限拒绝后仍可使用示例和上传', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator.mediaDevices, 'getUserMedia', { value: () => Promise.reject(new DOMException('denied', 'NotAllowedError')) });
  });
  await page.goto('/');
  await page.getByRole('button', { name: /开启摄像头/ }).click();
  await expect(page.getByRole('alert')).toContainText('摄像头权限被拒绝');
  await page.getByRole('button', { name: /使用示例图/ }).click();
  await expect(page.getByAltText('本次采集的照片预览')).toBeVisible();
  await page.locator('#file').setInputFiles({ name: 'bad.txt', mimeType: 'text/plain', buffer: Buffer.from('bad') });
  await expect(page.getByRole('alert')).toContainText('JPEG 或 PNG');
});

test('加载失败显示错误并提供重新连接入口', async ({ page }) => {
  await page.route('**/api/subjects', (route) => route.abort());
  await page.goto('/');
  await expect(page.getByRole('alert')).toContainText('无法连接服务');
  await page.getByRole('button', { name: /连接设置/ }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await page.getByRole('button', { name: '关闭连接设置' }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
});

test('主动拍照及切页后释放摄像头轨道', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /开启摄像头/ }).click();
  await expect(page.getByRole('button', { name: '拍摄照片' })).toBeEnabled();
  await page.evaluate(() => {
    const video = document.getElementById('video') as HTMLVideoElement;
    (window as Window & { testTrack?: MediaStreamTrack }).testTrack = (video.srcObject as MediaStream).getVideoTracks()[0];
  });
  await page.getByRole('button', { name: '拍摄照片' }).click();
  await expect(page.getByAltText('本次采集的照片预览')).toBeVisible();
  expect(await page.evaluate(() => (window as Window & { testTrack?: MediaStreamTrack }).testTrack?.readyState)).toBe('ended');
  await page.getByRole('button', { name: /开启摄像头/ }).click();
  await expect(page.getByRole('button', { name: '拍摄照片' })).toBeEnabled();
  await page.evaluate(() => {
    const video = document.getElementById('video') as HTMLVideoElement;
    (window as Window & { testTrack?: MediaStreamTrack }).testTrack = (video.srcObject as MediaStream).getVideoTracks()[0];
  });
  await page.getByRole('button', { name: /人员登记/ }).click();
  expect(await page.evaluate(() => (window as Window & { testTrack?: MediaStreamTrack }).testTrack?.readyState)).toBe('ended');
});

test('手机、平板和桌面布局不出现横向溢出', async ({ page }, info) => {
  await page.goto('/');
  await expect(page.locator('#connection')).toHaveText('服务已连接');
  for (const width of [360, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  }
  await page.setViewportSize({ width: info.project.name === 'mobile' ? 390 : 1440, height: 1080 });
  await page.screenshot({ path: info.outputPath('workspace.png'), fullPage: true });
});
