import asyncio


class Debouncer:
    def __init__(self, delay: float):
        self.delay = delay
        self._task = None

    async def debounce(self, coro):
        if self._task:
            self._task.cancel()
        self._task = asyncio.create_task(self._execute(coro))

    async def _execute(self, coro):
        try:
            await asyncio.sleep(self.delay)
            await coro()
        except asyncio.CancelledError:
            pass
