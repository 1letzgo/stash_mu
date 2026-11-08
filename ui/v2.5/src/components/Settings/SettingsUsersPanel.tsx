import React, { useState } from "react";
import { Button, Form, Table } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";
import { useCurrentUser } from "src/hooks/CurrentUser";
import { ModalComponent } from "../Shared/Modal";
import { LoadingIndicator } from "../Shared/LoadingIndicator";
import { SettingSection } from "./SettingSection";

const roles = [GQL.UserRole.Viewer, GQL.UserRole.Editor, GQL.UserRole.Admin];

const RoleSelect: React.FC<{
  value: GQL.UserRole;
  onChange: (v: GQL.UserRole) => void;
  disabled?: boolean;
}> = ({ value, onChange, disabled }) => {
  const intl = useIntl();
  return (
    <Form.Control
      as="select"
      className="input-control"
      value={value}
      disabled={disabled}
      onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
        onChange(e.currentTarget.value as GQL.UserRole)
      }
    >
      {roles.map((r) => (
        <option key={r} value={r}>
          {intl.formatMessage({ id: `config.users.roles.${r.toLowerCase()}` })}
        </option>
      ))}
    </Form.Control>
  );
};

interface IUserDialog {
  user?: GQL.UserDataFragment;
  onClose: (saved: boolean) => void;
}

const UserDialog: React.FC<IUserDialog> = ({ user, onClose }) => {
  const intl = useIntl();
  const Toast = useToast();
  const [username, setUsername] = useState(user?.username ?? "");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState(user?.role ?? GQL.UserRole.Viewer);
  const [saving, setSaving] = useState(false);

  const [createUser] = GQL.useUserCreateMutation();
  const [updateUser] = GQL.useUserUpdateMutation();

  const isNew = !user;
  const valid = username.trim() !== "" && (!isNew || password !== "");

  async function onSave() {
    setSaving(true);
    try {
      if (isNew) {
        await createUser({
          variables: { input: { username, password, role } },
        });
      } else {
        await updateUser({
          variables: {
            input: {
              id: user.id,
              username: username !== user.username ? username : undefined,
              password: password !== "" ? password : undefined,
              role: role !== user.role ? role : undefined,
            },
          },
        });
      }
      onClose(true);
    } catch (e) {
      Toast.error(e);
    } finally {
      setSaving(false);
    }
  }

  return (
    <ModalComponent
      show
      header={intl.formatMessage({
        id: isNew ? "config.users.add_user" : "config.users.edit_user",
      })}
      accept={{
        text: intl.formatMessage({ id: "actions.save" }),
        onClick: onSave,
      }}
      cancel={{
        text: intl.formatMessage({ id: "actions.cancel" }),
        variant: "secondary",
        onClick: () => onClose(false),
      }}
      onHide={() => onClose(false)}
      disabled={!valid}
      isRunning={saving}
    >
      <Form.Group>
        <Form.Label>
          <FormattedMessage id="config.general.auth.username" />
        </Form.Label>
        <Form.Control
          className="text-input"
          value={username}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
            setUsername(e.currentTarget.value)
          }
        />
      </Form.Group>
      <Form.Group>
        <Form.Label>
          <FormattedMessage id="config.general.auth.password" />
        </Form.Label>
        <Form.Control
          className="text-input"
          type="password"
          autoComplete="new-password"
          value={password}
          placeholder={
            isNew
              ? undefined
              : intl.formatMessage({ id: "config.users.password_unchanged" })
          }
          onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
            setPassword(e.currentTarget.value)
          }
        />
      </Form.Group>
      <Form.Group>
        <Form.Label>
          <FormattedMessage id="config.users.role" />
        </Form.Label>
        <RoleSelect value={role} onChange={setRole} />
        <Form.Text className="text-muted">
          <FormattedMessage
            id={`config.users.role_desc.${role.toLowerCase()}`}
          />
        </Form.Text>
      </Form.Group>
    </ModalComponent>
  );
};

export const SettingsUsersPanel: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();
  const { me } = useCurrentUser();
  const { data, loading, error, refetch } = GQL.useFindUsersQuery({
    fetchPolicy: "network-only",
  });
  const [destroyUser] = GQL.useUserDestroyMutation();
  const [generateKey] = GQL.useUserGenerateApiKeyMutation();

  const [editing, setEditing] = useState<GQL.UserDataFragment | null>();
  const [deleting, setDeleting] = useState<GQL.UserDataFragment>();
  const [newKey, setNewKey] = useState<{ username: string; key: string }>();

  if (error) return <h1>{error.message}</h1>;
  if (loading) return <LoadingIndicator />;

  const users = data?.findUsers ?? [];

  async function onDelete() {
    if (!deleting) return;
    try {
      await destroyUser({ variables: { id: deleting.id } });
      refetch();
    } catch (e) {
      Toast.error(e);
    } finally {
      setDeleting(undefined);
    }
  }

  async function onGenerateKey(u: GQL.UserDataFragment, clear: boolean) {
    try {
      const result = await generateKey({
        variables: { input: { id: u.id, clear } },
      });
      if (!clear && result.data) {
        setNewKey({
          username: u.username,
          key: result.data.userGenerateAPIKey,
        });
      } else {
        Toast.success(
          intl.formatMessage({ id: "config.users.api_key_cleared" })
        );
      }
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <SettingSection headingID="config.categories.users">
      <div className="setting-group">
        <div className="sub-heading mb-3">
          <FormattedMessage id="config.users.description" />
        </div>
        <Table striped size="sm" className="users-table">
          <thead>
            <tr>
              <th>
                <FormattedMessage id="config.general.auth.username" />
              </th>
              <th>
                <FormattedMessage id="config.users.role" />
              </th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>
                  {u.username}
                  {u.id === me?.id && (
                    <span className="text-muted">
                      {" "}
                      (<FormattedMessage id="config.users.you" />)
                    </span>
                  )}
                </td>
                <td>
                  <FormattedMessage
                    id={`config.users.roles.${u.role.toLowerCase()}`}
                  />
                </td>
                <td className="text-right">
                  <Button size="sm" onClick={() => setEditing(u)}>
                    <FormattedMessage id="actions.edit" />
                  </Button>{" "}
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => onGenerateKey(u, false)}
                  >
                    <FormattedMessage id="config.general.auth.generate_api_key" />
                  </Button>{" "}
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => onGenerateKey(u, true)}
                  >
                    <FormattedMessage id="config.general.auth.clear_api_key" />
                  </Button>{" "}
                  <Button
                    size="sm"
                    variant="danger"
                    disabled={u.id === me?.id || u.id === "1"}
                    onClick={() => setDeleting(u)}
                  >
                    <FormattedMessage id="actions.delete" />
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        <Button onClick={() => setEditing(null)}>
          <FormattedMessage id="config.users.add_user" />
        </Button>
      </div>

      {editing !== undefined && (
        <UserDialog
          user={editing ?? undefined}
          onClose={(saved) => {
            setEditing(undefined);
            if (saved) refetch();
          }}
        />
      )}

      <ModalComponent
        show={!!deleting}
        header={intl.formatMessage({ id: "config.users.delete_user" })}
        accept={{
          text: intl.formatMessage({ id: "actions.delete" }),
          variant: "danger",
          onClick: onDelete,
        }}
        cancel={{
          text: intl.formatMessage({ id: "actions.cancel" }),
          variant: "secondary",
          onClick: () => setDeleting(undefined),
        }}
        onHide={() => setDeleting(undefined)}
      >
        <FormattedMessage
          id="config.users.delete_user_confirm"
          values={{ username: deleting?.username }}
        />
      </ModalComponent>

      <ModalComponent
        show={!!newKey}
        header={intl.formatMessage({ id: "config.general.auth.api_key" })}
        accept={{
          text: intl.formatMessage({ id: "actions.close" }),
          onClick: () => setNewKey(undefined),
        }}
        onHide={() => setNewKey(undefined)}
      >
        <p>
          <FormattedMessage
            id="config.users.new_api_key"
            values={{ username: newKey?.username }}
          />
        </p>
        <pre className="text-break" style={{ whiteSpace: "pre-wrap" }}>
          {newKey?.key}
        </pre>
      </ModalComponent>
    </SettingSection>
  );
};
