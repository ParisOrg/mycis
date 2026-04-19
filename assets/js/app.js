import "basecoat-css/basecoat";
import "basecoat-css/sidebar";
import "basecoat-css/dropdown-menu";
import "basecoat-css/select";
import "basecoat-css/toast";
import flatpickr from "flatpickr";

function bindSidebarTriggers() {
  const sidebar = document.querySelector("#sidebar");
  const triggers = [...document.querySelectorAll("[data-sidebar-trigger]")];

  if (!sidebar || triggers.length === 0) {
    return;
  }

  const syncExpandedState = () => {
    const expanded = sidebar.getAttribute("aria-hidden") !== "true";
    triggers.forEach((trigger) => {
      trigger.setAttribute("aria-expanded", expanded ? "true" : "false");
    });
  };

  triggers.forEach((trigger) => {
    trigger.addEventListener("click", () => {
      document.dispatchEvent(new CustomEvent("basecoat:sidebar", { detail: { id: "sidebar" } }));
    });
  });

  const observer = new MutationObserver(syncExpandedState);
  observer.observe(sidebar, {
    attributes: true,
    attributeFilter: ["aria-hidden"],
  });

  syncExpandedState();
}

function bindThemeSwitchers() {
  document.querySelectorAll(".theme-switcher").forEach((button) => {
    button.addEventListener("click", () => {
      document.dispatchEvent(new CustomEvent("basecoat:theme", { detail: { mode: "toggle" } }));
    });
  });
}

function bindDialogs() {
  const dialogs = [...document.querySelectorAll("dialog[data-dialog-auto-open], dialog[id]")];
  if (dialogs.length === 0) {
    return;
  }

  const openDialog = (dialog) => {
    if (!dialog || dialog.open || typeof dialog.showModal !== "function") {
      return;
    }
    dialog.showModal();
  };

  const closeDialog = (dialog) => {
    if (!dialog || !dialog.open) {
      return;
    }
    dialog.close();
  };

  document.querySelectorAll("[data-dialog-open]").forEach((trigger) => {
    trigger.addEventListener("click", () => {
      openDialog(document.getElementById(trigger.dataset.dialogOpen));
    });
  });

  document.querySelectorAll("[data-dialog-close]").forEach((trigger) => {
    trigger.addEventListener("click", () => {
      const target = trigger.dataset.dialogClose
        ? document.getElementById(trigger.dataset.dialogClose)
        : trigger.closest("dialog");
      closeDialog(target);
    });
  });

  dialogs.forEach((dialog) => {
    if (dialog.dataset.dialogAutoOpen === "true") {
      openDialog(dialog);
    }

    dialog.addEventListener("click", (event) => {
      if (event.target === dialog) {
        closeDialog(dialog);
      }
    });
  });
}

function bindEditUserTriggers() {
  const dialog = document.getElementById("edit-user-dialog");
  if (!dialog) {
    return;
  }

  const fields = {
    userID: dialog.querySelector("#edit-user-id"),
    hiddenEmail: dialog.querySelector("#edit-user-email-hidden"),
    name: dialog.querySelector("#edit-user-name"),
    email: dialog.querySelector("#edit-user-email"),
    role: dialog.querySelector("#edit-user-role"),
    password: dialog.querySelector("#edit-user-password"),
  };

  document.querySelectorAll("[data-edit-user-trigger]").forEach((trigger) => {
    trigger.addEventListener("click", () => {
      if (fields.userID) fields.userID.value = trigger.dataset.userId || "";
      if (fields.hiddenEmail) fields.hiddenEmail.value = trigger.dataset.userEmail || "";
      if (fields.name) fields.name.value = trigger.dataset.userName || "";
      if (fields.email) fields.email.value = trigger.dataset.userEmail || "";
      if (fields.role) fields.role.value = trigger.dataset.userRole || "";
      if (fields.password) fields.password.value = "";
    });
  });
}

function countSelected(checkboxes) {
  return checkboxes.filter((checkbox) => checkbox.checked).length;
}

function syncBulkState(checkboxes, toggle, countNode, bar) {
  const selected = countSelected(checkboxes);

  if (countNode) {
    countNode.textContent = `${selected} selected`;
  }
  if (bar) {
    bar.classList.toggle("bulk-bar--visible", selected > 0);
  }
  if (toggle) {
    toggle.checked = selected > 0 && selected === checkboxes.length;
    toggle.indeterminate = selected > 0 && selected < checkboxes.length;
  }
}

function bindBulkSelection(form, checkboxes, toggle, countNode, bar) {
  if (toggle) {
    toggle.addEventListener("change", () => {
      checkboxes.forEach((checkbox) => {
        checkbox.checked = toggle.checked;
      });
      syncBulkState(checkboxes, toggle, countNode, bar);
    });
  }

  checkboxes.forEach((checkbox) => {
    checkbox.addEventListener("change", () => {
      syncBulkState(checkboxes, toggle, countNode, bar);
    });
  });

  const clearButton = form.querySelector("[data-bulk-clear]");
  if (!clearButton) {
    return;
  }

  clearButton.addEventListener("click", () => {
    checkboxes.forEach((checkbox) => {
      checkbox.checked = false;
    });
    syncBulkState(checkboxes, toggle, countNode, bar);
  });
}

function bindBulkSubmit(form) {
  const actionInput = form.querySelector('input[name="action"]');
  let lastChanged = null;

  form.querySelectorAll("[data-bulk-action]").forEach((field) => {
    field.addEventListener("change", () => {
      lastChanged = field;
    });
  });

  form.addEventListener("submit", (event) => {
    if (!lastChanged || !lastChanged.value) {
      event.preventDefault();
      return;
    }

    actionInput.value = lastChanged.dataset.bulkAction;
    const hiddenField = form.querySelector(`input[name="${lastChanged.dataset.bulkField}"]`);
    if (hiddenField) {
      hiddenField.value = lastChanged.value;
    }
  });
}

function bindBulkForms() {
  const bulkForms = document.querySelectorAll("[data-bulk-form]");
  bulkForms.forEach((form) => {
    const bar = form.querySelector("[data-bulk-bar]");
    const checkboxes = [...form.querySelectorAll("input[type='checkbox'][name='item_ids']")];
    const toggle = form.querySelector("[data-select-all]");
    const countNode = form.querySelector("[data-selected-count]");

    bindBulkSelection(form, checkboxes, toggle, countNode, bar);
    syncBulkState(checkboxes, toggle, countNode, bar);
    bindBulkSubmit(form);
  });
}

function bindDatePickers() {
  const dateFields = document.querySelectorAll("[data-flatpickr]");
  if (dateFields.length === 0) return;

  const isDark = document.documentElement.classList.contains("dark");

  const commonConfig = {
    dateFormat: "Y-m-d",
    allowInput: true,
    theme: isDark ? "dark" : undefined,
  };

  dateFields.forEach((field) => {
    const config = { ...commonConfig };

    if (field.dataset.flatpickrMin) {
      config.minDate = field.dataset.flatpickrMin;
    }

    const fp = flatpickr(field, config);

    if (field.dataset.flatpickrSync) {
      const syncTarget = document.querySelector(field.dataset.flatpickrSync);
      if (syncTarget) {
        fp.set("onChange", (selectedDates) => {
          if (selectedDates[0]) {
            const syncPicker = syncTarget._flatpickr;
            if (syncPicker) {
              syncPicker.set("minDate", selectedDates[0]);
            }
          }
        });
      }
    }
  });
}

function ready() {
  bindSidebarTriggers();
  bindThemeSwitchers();
  bindDialogs();
  bindEditUserTriggers();
  bindBulkForms();
  bindDatePickers();
}

document.addEventListener("DOMContentLoaded", ready);
