## Simple CRU~~D~~ api on Go w/ Mongo

Simply start `docker compose up` and go to [localhost:3000](http://localhost:3000/).

| URL      | Method | Description                                                   |
| -------- | ------ | ------------------------------------------------------------- |
| `/`      | `GET`  | Get cached list of all entities                               |
| `/`      | `POST` | Create new entity. Example payload: `{"title": "some title"}` |
| `/{id}/` | `PUT`  | Update existing entity. Example payload: `same as above`      |
