BUMP_PART ?=
PYTEST_COV ?= xml
setup:
	python3 -m pip install poetry

install:
	poetry install

test:
	poetry run pytest -ssv --cov --cov-report=$(PYTEST_COV)

build:
	poetry build

publish: build
	poetry publish

format:
	poetry run ruff format pyre_tui/ tests/
	poetry run ruff --fix pyre_tui/ tests/

lint:
	poetry run ruff pyre_tui/ tests/
	poetry run mypy pyre_tui/ tests/

bump:
	poetry run bump-my-version bump $(BUMP_PART)
