import { expect, test, type Page } from "@playwright/test";

async function createRoom(page: Page, roomName: string, name: string) {
  await page.goto("/");
  await page.getByLabel("Room name").fill(roomName);
  await page.getByLabel("Your name").fill(name);
  await page.getByRole("button", { name: /create room/i }).last().click();
  await expect(page).toHaveURL(/\/room\/[A-Z0-9]{6}$/);
  return (await page.getByTestId("room-code").textContent())!.trim();
}

async function joinByInviteLink(page: Page, code: string, name: string) {
  await page.goto(`/room/${code}`);
  await expect(page).toHaveURL(new RegExp(`join=${code}`));
  await expect(page.locator("#join-room-code")).toHaveValue(code);
  await page.locator("#join-display-name").fill(name);
  await page.getByRole("button", { name: /join room/i }).last().click();
  await expect(page).toHaveURL(new RegExp(`/room/${code}$`));
}

test("full planning session between a host and a guest", async ({ browser }) => {
  const hostCtx = await browser.newContext();
  const guestCtx = await browser.newContext();
  const host = await hostCtx.newPage();
  const guest = await guestCtx.newPage();

  const code = await createRoom(host, "Sprint 42", "Alice");
  await joinByInviteLink(guest, code, "Bob");

  // Both are connected and see each other.
  for (const page of [host, guest]) {
    await expect(page.getByTestId("connection-status")).toContainText(/live|connected/i);
    await expect(page.getByText("Alice")).toBeVisible();
    await expect(page.getByText("Bob")).toBeVisible();
  }
  await expect(guest.getByTestId("waiting-host")).toBeVisible();
  await expect(guest.getByTestId("host-controls")).toHaveCount(0);

  // Host starts a round; guest receives it in real time.
  await host.locator("#story-title").fill("Login page");
  await host.getByTestId("start-round").click();
  await expect(guest.getByTestId("story-title")).toHaveText("Login page");

  // Both vote; own card stays selected and the host sees the progress.
  await host.getByTestId("card-5").click();
  await guest.getByTestId("card-8").click();
  await guest.getByTestId("card-5").click(); // change of mind
  await expect(guest.getByTestId("card-5")).toHaveAttribute("aria-pressed", "true");
  await expect(host.getByTestId("voted-count")).toContainText("2/2");

  // Reloading keeps the session, host role and own vote.
  await host.reload();
  await expect(host.getByTestId("host-controls")).toBeVisible();
  await expect(host.getByTestId("card-5")).toHaveAttribute("aria-pressed", "true");

  // Reveal: both see the same consensus.
  await host.getByTestId("reveal").click();
  for (const page of [host, guest]) {
    await expect(page.getByTestId("consensus")).toBeVisible();
    await expect(page.getByTestId("average")).toHaveText("5");
  }

  // Revote clears the votes of the same story.
  await host.getByTestId("revote").click();
  await expect(guest.getByTestId("round-status")).toHaveText("Voting");
  await expect(guest.getByTestId("card-5")).toHaveAttribute("aria-pressed", "false");
  await expect(guest.getByTestId("story-title")).toHaveText("Login page");

  // Split vote then reveal: no consensus, average of numeric votes only.
  await host.getByTestId("card-3").click();
  await guest.getByTestId("card-?").click();
  await expect(host.getByTestId("voted-count")).toContainText("2/2");
  await host.getByTestId("reveal").click();
  await expect(guest.getByTestId("average")).toHaveText("3");

  // Next story replaces the revealed one.
  await host.locator("#story-title").fill("Password reset");
  await host.getByTestId("start-round").click();
  await expect(guest.getByTestId("story-title")).toHaveText("Password reset");
  await expect(guest.getByTestId("round-status")).toHaveText("Voting");

  // Host leaves: Bob becomes host.
  host.on("dialog", (d) => d.accept());
  await host.getByTestId("leave").click();
  await expect(host).toHaveURL(/\/($|\?)/);
  await expect(guest.getByText("Alice")).toHaveCount(0);
  await expect(guest.getByTestId("host-controls")).toBeVisible();

  await hostCtx.close();
  await guestCtx.close();
});

test("joining an unknown room shows an error", async ({ page }) => {
  await page.goto("/?join=ZZZZZZ");
  await page.locator("#join-display-name").fill("Carol");
  await page.getByRole("button", { name: /join room/i }).last().click();
  await expect(page.getByRole("alert")).toContainText(/not found/i);
});
