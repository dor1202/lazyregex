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
	poetry run ruff format mochi_re/ tests/
	poetry run ruff --fix mochi_re/ tests/

lint:
	poetry run ruff mochi_re/ tests/
	poetry run mypy mochi_re/ tests/

bump:
	poetry run bump-my-version bump $(BUMP_PART)
